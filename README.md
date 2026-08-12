# ss-patcher

## AI Disclosure

The code was initially created almost entirely by AI and then human tested with plenty of trial and error.

## Purpose

- Kills any running `SteelSeries*` processes
- Connect to `C:\ProgramData\SteelSeries\GG\db\database.db`
- Create the `settings` table if it does not exist
- Create the `loadoutsEligible` key with a value of `true` if it does not exist (set it to `true` if it does exist)

After this has completed, reopen SteelSeries and Quickset should be available.
