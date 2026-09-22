-- users.mobile: unique index for lookup performance and uniqueness (NULLs allowed)
-- migrate: requires users
ALTER TABLE users ADD UNIQUE INDEX uk_users_mobile (mobile);

-- alumni_profiles.email: index for lookup by email
ALTER TABLE alumni_profiles ADD INDEX idx_alumni_email (email);
