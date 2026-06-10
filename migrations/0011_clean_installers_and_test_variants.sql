-- 0011: further false-positive cleanup, mirroring internal/games/discord.go.
--  - strip installer/updater/config executables (e.g. setup.exe matched
--    "Hidden & Dangerous 2" whenever anyone ran any installer);
--  - neutralize detection for non-retail editions (test servers, PTR, betas,
--    playtests) whose executables overlap and steal the main game's presence
--    (e.g. "PUBG: Test Server" via execpubg.exe).

BEGIN;

WITH banned AS (
    SELECT unnest(ARRAY[
        'setup.exe', 'setup_x64.exe', 'setup_x86.exe',
        'install.exe', 'installer.exe', 'uninstall.exe', 'unins000.exe',
        'update.exe', 'updater.exe', 'launcher_updater.exe', 'patcher.exe',
        'config.exe', 'settings.exe', 'run.exe', 'prerequisites.exe'
    ]) AS exe
)
UPDATE games g
SET executable_names = COALESCE((
    SELECT array_agg(e ORDER BY e)
    FROM unnest(g.executable_names) AS e
    WHERE e NOT IN (SELECT exe FROM banned)
), '{}')
WHERE EXISTS (
    SELECT 1 FROM unnest(g.executable_names) AS e
    WHERE e IN (SELECT exe FROM banned)
);

UPDATE games
SET executable_names = '{}'
WHERE executable_names <> '{}'
  AND name ~* '(test server|public test|play ?test|closed test|open test|closed beta|open beta|public beta|beta test|experimental|\yPTR\y|\yPTS\y)';

COMMIT;
