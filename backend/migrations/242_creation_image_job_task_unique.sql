-- One provider task id per user so retries cannot attach the wrong history row.
CREATE UNIQUE INDEX IF NOT EXISTS creation_image_jobs_user_provider_task_unique_idx
    ON creation_image_jobs (user_id, provider_task_id)
    WHERE provider_task_id IS NOT NULL AND provider_task_id <> '';
