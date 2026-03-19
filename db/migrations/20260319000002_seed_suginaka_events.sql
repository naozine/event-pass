-- +goose Up

-- Suginaka Private School Fair 2025 (4 events)
INSERT INTO events (code, title, description, venue, event_date, capacity, color_bg, color_text, group_name, is_published, custom_fields) VALUES
('T1', '第１部', '杉並中野私立中学高等学校フェア', '', '2026-05-11 10:00:00', 0, '#C4FFDC', '#000000', '杉並中野私立中学高等学校フェア 2025', 1,
 '[{"key": "区分", "value": "完全予約制"}, {"key": "時間帯", "value": "10:00-11:20"}, {"key": "サブカラー", "value": "#ECFFF3"}]'),
('T2', '第２部', '杉並中野私立中学高等学校フェア', '', '2026-05-11 11:40:00', 0, '#C8EBFD', '#000000', '杉並中野私立中学高等学校フェア 2025', 1,
 '[{"key": "区分", "value": "完全予約制"}, {"key": "時間帯", "value": "11:40-13:00"}, {"key": "サブカラー", "value": "#EBF4FE"}]'),
('T3', '第３部', '杉並中野私立中学高等学校フェア', '', '2026-05-11 13:30:00', 0, '#FEFCA5', '#000000', '杉並中野私立中学高等学校フェア 2025', 1,
 '[{"key": "区分", "value": "完全予約制"}, {"key": "時間帯", "value": "13:30-14:50"}, {"key": "サブカラー", "value": "#FFFAD0"}]'),
('T4', '第４部', '杉並中野私立中学高等学校フェア', '', '2026-05-11 15:10:00', 0, '#F4BBB8', '#000000', '杉並中野私立中学高等学校フェア 2025', 1,
 '[{"key": "区分", "value": "完全予約制"}, {"key": "時間帯", "value": "15:10-16:30"}, {"key": "サブカラー", "value": "#FBE8E7"}]');

-- Sample registration for demo attendee
INSERT INTO registrations (event_id, user_id, name, custom_fields)
SELECT e.id, u.id, u.name, '[]'
FROM events e, users u
WHERE e.code = 'T1' AND e.group_name = '杉並中野私立中学高等学校フェア 2025' AND u.email = 'attendee@example.com';

-- +goose Down
DELETE FROM registrations WHERE event_id IN (SELECT id FROM events WHERE group_name = '杉並中野私立中学高等学校フェア 2025');
DELETE FROM events WHERE group_name = '杉並中野私立中学高等学校フェア 2025';
