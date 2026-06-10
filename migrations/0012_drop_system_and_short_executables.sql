-- 0012: drop Windows system binaries and very short executable stems (1-2
-- chars) that collide with routine OS activity, mirroring the seed-time guards
-- in internal/games/discord.go. Example: "Steel Circus" was detected via
-- sc.exe (the Service Control tool).

BEGIN;

WITH banned AS (
    SELECT unnest(ARRAY[
        'sc.exe', 'net.exe', 'net1.exe', 'reg.exe', 'ping.exe', 'ftp.exe',
        'find.exe', 'findstr.exe', 'sort.exe', 'more.exe', 'tree.exe',
        'mode.exe', 'where.exe', 'whoami.exe', 'tasklist.exe', 'taskkill.exe',
        'control.exe', 'regedit.exe', 'rundll32.exe', 'svchost.exe',
        'conhost.exe', 'dllhost.exe', 'werfault.exe', 'explorer.exe',
        'notepad.exe', 'mmc.exe', 'powershell.exe', 'pwsh.exe', 'wscript.exe',
        'cscript.exe', 'mshta.exe'
    ]) AS exe
)
UPDATE games g
SET executable_names = COALESCE((
    SELECT array_agg(e ORDER BY e)
    FROM unnest(g.executable_names) AS e
    WHERE e NOT IN (SELECT exe FROM banned)
      AND e !~ '^[a-z0-9]{1,2}\.'
), '{}')
WHERE EXISTS (
    SELECT 1 FROM unnest(g.executable_names) AS e
    WHERE e IN (SELECT exe FROM banned) OR e ~ '^[a-z0-9]{1,2}\.'
);

COMMIT;
