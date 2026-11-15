ALTER TABLE users
    ADD COLUMN locations TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN roles TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN interests TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN min_experience_years INT,
    ADD COLUMN max_experience_years INT,
    ADD CONSTRAINT users_experience_check CHECK (
        (min_experience_years IS NULL OR min_experience_years >= 0)
        AND (max_experience_years IS NULL OR max_experience_years >= 0)
        AND (
            min_experience_years IS NULL
            OR max_experience_years IS NULL
            OR min_experience_years <= max_experience_years
        )
    );
