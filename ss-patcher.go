package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/process"

	// Register the pure-Go SQLite driver with the database/sql package
	_ "modernc.org/sqlite"
)

// isAdmin checks if the current process has administrative privileges on Windows
func isAdmin() bool {
	if runtime.GOOS != "windows" {
		return true // Skip check on non-Windows platforms
	}

	// On Windows, checking if we can write to a protected directory is a reliable way to check admin rights
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

// requestAdminPrivileges re-launches the current executable with elevated rights
func requestAdminPrivileges() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find executable path: %w", err)
	}

	var cmdArgs []string

	// Check if there are any command-line arguments to pass along
	if len(os.Args) > 1 {
		args := strings.Join(os.Args[1:], " ")

		// Include -ArgumentList only when arguments exist
		cmdArgs = []string{
			"Start-Process",
			"-FilePath", fmt.Sprintf(`"%s"`, exe),
			"-ArgumentList", fmt.Sprintf(`"%s"`, args),
			"-Verb", "RunAs",
		}
	} else {
		// Omit -ArgumentList entirely if no arguments are provided
		cmdArgs = []string{
			"Start-Process",
			"-FilePath", fmt.Sprintf(`"%s"`, exe),
			"-Verb", "RunAs",
		}
	}

	// Execute PowerShell with the safe arguments list
	cmd := exec.Command("powershell", append([]string{"-Command"}, cmdArgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("UAC elevation prompt rejected: %w", err)
	}

	// Exit the current non-admin process since the admin one is now running
	// os.Exit(0)
	return nil
}

// killProcessesByPattern finds and terminates all processes matching a wildcard pattern
func killProcessesByPattern(pattern string) error {
	// 1. Retrieve a list of all currently running processes
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to fetch processes: %w", err)
	}

	fmt.Printf("Searching for processes matching pattern: %s\n", pattern)

	for _, p := range processes {
		// 2. Get the name of the executable for each process
		name, err := p.Name()
		if err != nil {
			// Skip processes that we don't have permission to read (e.g., system processes)
			continue
		}

		// 3. Match the process name against our wildcard pattern
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return fmt.Errorf("invalid wildcard pattern: %w", err)
		}

		if matched {
			pid := p.Pid
			fmt.Printf("Found matching process: %s (PID: %d). Terminating...\n", name, pid)

			// 4. Find the OS-level process object and kill it
			osProcess, err := os.FindProcess(int(pid))
			fmt.Printf("DEBUG: Found OS process for PID %d: %s\n", pid, name)
			if err != nil {
				fmt.Printf("DEBUG: Failed to find OS process for PID %d: %s\n", pid, name)
				// log.Printf("Could not find OS process for PID %d: %v\n", pid, err)
				continue
			}

			if err := osProcess.Kill(); err != nil {
				fmt.Printf("DEBUG: Failed to kill process %s (PID: %d): %v\n", name, pid, err)
				// log.Printf("Failed to kill process %s (PID: %d): %v\n", name, pid, err)
			} else {
				fmt.Printf("Successfully killed process %s (PID: %d)\n", name, pid)
			}
		}
	}

	return nil
}

// updateDatabase ensures the settings table exists and updates the target row
func updateDatabase(dbPath string) {
	// Verify the database file physically exists before opening
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Error: Database file not found at %s\n", dbPath)
		os.Exit(1)
	}

	fmt.Println("Connecting to database...")

	// Open connection using the registered sqlite3 driver string
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Failed to open connection: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 1. Create the settings table if it does not exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT NOT NULL PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		fmt.Printf("Failed to create table: %v\n", err)
		return
	}

	// 2. Insert or update the target row using SQLite's native CURRENT_TIMESTAMP
	upsertSQL := `
	INSERT INTO settings (key, value, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT (key) DO UPDATE
	SET value = excluded.value, updated_at = excluded.updated_at;`

	_, err = db.Exec(upsertSQL, "loadoutsEligible", "true")
	if err != nil {
		fmt.Printf("Failed to update database record: %v\n", err)
		return
	}

	fmt.Println("Database successfully updated!")
}

func main() {
	if runtime.GOOS == "windows" && !isAdmin() {
		fmt.Println("Administrative privileges required. Prompting for elevation...")
		if err := requestAdminPrivileges(); err != nil {
			log.Fatalf("Failed to elevate privileges: %v", err)
		}
	}

	pattern := "SteelSeries*"
	if err := killProcessesByPattern(pattern); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	targetDB := `C:\ProgramData\SteelSeries\GG\db\database.db`
	updateDatabase(targetDB)
}
