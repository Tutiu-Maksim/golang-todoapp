BEGIN;

ALTER TABLE todoapp.tasks
    DROP CONSTRAINT tasks_description_check,
    ADD CONSTRAINT tasks_title_check1
    CHECK (char_length(title) BETWEEN 1 AND 1000);

COMMIT;
