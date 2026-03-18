-- +goose Up

-- School festa events (web-ticket-nextjs style with custom_fields)
INSERT INTO events (title, description, venue, event_date, capacity, is_published, custom_fields) VALUES
('Kaisei', 'School presentation and campus tour.', 'Tokyo Private Boys School Festa 2026', '2026-06-22 10:00:00', 40, 1,
 '[{"key":"Event","value":"Tokyo Private Boys School Festa 2026"},{"key":"Section","value":"School Presentation"},{"key":"Floor","value":"3F"},{"key":"Room","value":"301"},{"key":"Time Slot","value":"10:00 - 10:30"}]'),
('Azabu', 'School presentation and Q&A session.', 'Tokyo Private Boys School Festa 2026', '2026-06-22 10:00:00', 40, 1,
 '[{"key":"Event","value":"Tokyo Private Boys School Festa 2026"},{"key":"Section","value":"School Presentation"},{"key":"Floor","value":"3F"},{"key":"Room","value":"302"},{"key":"Time Slot","value":"10:00 - 10:30"}]'),
('Musashi', 'Hands-on workshop and school introduction.', 'Tokyo Private Boys School Festa 2026', '2026-06-22 11:00:00', 30, 1,
 '[{"key":"Event","value":"Tokyo Private Boys School Festa 2026"},{"key":"Section","value":"Workshop"},{"key":"Floor","value":"4F"},{"key":"Room","value":"401"},{"key":"Time Slot","value":"11:00 - 11:45"}]'),
('Takushoku Univ. Daiichi', 'Trial class experience.', 'Tokyo Private Boys School Festa 2026', '2026-06-22 11:00:00', 25, 1,
 '[{"key":"Event","value":"Tokyo Private Boys School Festa 2026"},{"key":"Section","value":"Trial Class"},{"key":"Floor","value":"4F"},{"key":"Room","value":"402"},{"key":"Time Slot","value":"11:00 - 11:45"}]'),
('Seigakuin', 'Afternoon presentation and panel discussion.', 'Tokyo Private Boys School Festa 2026', '2026-06-22 13:00:00', 40, 1,
 '[{"key":"Event","value":"Tokyo Private Boys School Festa 2026"},{"key":"Section","value":"School Presentation"},{"key":"Floor","value":"2F"},{"key":"Room","value":"201"},{"key":"Time Slot","value":"13:00 - 13:30"}]');

-- Festa registrations for demo attendee
INSERT INTO registrations (event_id, user_id, name, custom_fields)
SELECT e.id, u.id, u.name, '[]'
FROM events e, users u
WHERE e.title = 'Kaisei' AND u.email = 'attendee@example.com';

INSERT INTO registrations (event_id, user_id, name, custom_fields)
SELECT e.id, u.id, u.name, '[]'
FROM events e, users u
WHERE e.title = 'Musashi' AND u.email = 'attendee@example.com';

-- +goose Down
DELETE FROM registrations WHERE event_id IN (SELECT id FROM events WHERE custom_fields != '[]');
DELETE FROM events WHERE custom_fields != '[]';
