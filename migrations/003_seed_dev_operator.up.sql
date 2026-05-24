INSERT INTO operators (
    email, password_hash, name, role, is_active
) VALUES (
    'admin@airdanapi.local',
    '$argon2id$v=19$m=65536,t=3,p=2$BenKzai2qoO7SeJ2h+Cmxg$jmK5Y3ZfaZd3TuVDBJre5Ma6wulztr2lqvDhML6A0Qg',
    'Admin Integrator',
    'AdminFull',
    TRUE
) ON DUPLICATE KEY UPDATE
    password_hash = VALUES(password_hash),
    name = VALUES(name),
    role = VALUES(role),
    is_active = VALUES(is_active);
