BEGIN;

ALTER TABLE todoapp.tasks DROP CONSTRAINT tasks_title_check1;

UPDATE todoapp.tasks SET description = NULL WHERE description = '';

ALTER TABLE todoapp.tasks
    ADD CONSTRAINT tasks_description_check
    CHECK (char_length(description) BETWEEN 1 AND 1000);

COMMIT;
