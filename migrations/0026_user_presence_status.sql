-- 0026: User-chosen presence status. 'online' = derive from connection/session
-- (default, no behavior change). 'invisible'/'offline' = appear offline to others.
BEGIN;
ALTER TABLE users ADD COLUMN presence_status text NOT NULL DEFAULT 'online'
    CHECK (presence_status IN ('online', 'invisible', 'offline'));
COMMIT;
