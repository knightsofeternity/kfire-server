-- 0009_clean_generic_executables.sql - strip non-discriminating executables
-- from the catalog so the client stops reporting false positives (e.g. a Unity
-- crash handler or a VPN TAP adapter idling in the background looked like a
-- running game). Mirrors the seed-time filter in internal/games/discord.go.

BEGIN;

-- A basename shared by more than 3 games can't identify a single game; combine
-- those with a static blocklist of known helper/runtime/driver binaries.
WITH shared AS (
    SELECT exe
    FROM (SELECT unnest(executable_names) AS exe FROM games) t
    GROUP BY exe
    HAVING count(*) > 3
),
banned AS (
    SELECT exe FROM shared
    UNION
    SELECT unnest(ARRAY[
        'unitycrashhandler64.exe', 'unitycrashhandler32.exe', 'unitycrashhandler.exe',
        'crashhandler.exe', 'crashreporter.exe', 'crashpad_handler.exe',
        'ueprereqsetup_x64.exe', 'ue4prereqsetup_x64.exe',
        'tap.exe',
        'game.exe', 'launcher.exe', 'start.exe', 'play.exe', 'autorun.exe',
        'vc_redist.x64.exe', 'vc_redist.x86.exe', 'dxsetup.exe', 'dotnet.exe',
        'python.exe', 'pythonw.exe', 'java.exe', 'javaw.exe', 'mono.exe',
        'node.exe', 'nw.exe', 'cmd.exe'
    ])
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

COMMIT;
