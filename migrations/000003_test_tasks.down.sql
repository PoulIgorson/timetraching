DELETE FROM tasks
WHERE id IN (
    SELECT id
    FROM tasks
    ORDER BY created DESC
    LIMIT 10
);
