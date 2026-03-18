-- +goose Up

-- Demo admin user
INSERT OR IGNORE INTO users (email, name, role, is_active)
VALUES ('admin@example.com', 'Admin User', 'admin', 1);

-- Demo attendee user
INSERT OR IGNORE INTO users (email, name, role, is_active)
VALUES ('attendee@example.com', 'Jane Doe', 'viewer', 1);

-- Sample events
INSERT INTO events (title, description, venue, event_date, capacity, is_published) VALUES
('Tech Conference 2026', 'Annual technology conference featuring talks on AI, cloud computing, and modern web development.', 'Grand Convention Center, Hall A', '2026-06-15 09:00:00', 200, 1),
('Web Development Workshop', 'Hands-on workshop covering Go, htmx, and Tailwind CSS for building modern web applications.', 'Innovation Hub, Room 301', '2026-07-20 13:00:00', 30, 1),
('Startup Pitch Night', 'An evening of startup pitches and networking. Meet founders and investors in the local tech scene.', 'Downtown Coworking Space', '2026-08-05 18:00:00', 50, 1),
('Cloud Architecture Seminar', 'Deep dive into cloud-native architecture patterns and best practices for scalable systems.', 'TechPark Building B, Auditorium', '2026-09-10 10:00:00', 100, 1),
('Design Systems Workshop', 'Learn how to build and maintain a design system for your product team.', 'Creative Studio, 5F', '2026-10-01 14:00:00', 25, 0);

-- Sample registrations (attendee@example.com registers for first 2 events)
INSERT INTO registrations (event_id, user_id, name)
SELECT e.id, u.id, u.name
FROM events e, users u
WHERE e.title = 'Tech Conference 2026' AND u.email = 'attendee@example.com';

INSERT INTO registrations (event_id, user_id, name)
SELECT e.id, u.id, u.name
FROM events e, users u
WHERE e.title = 'Web Development Workshop' AND u.email = 'attendee@example.com';

-- +goose Down
DELETE FROM registrations;
DELETE FROM events;
DELETE FROM users WHERE email IN ('admin@example.com', 'attendee@example.com');
