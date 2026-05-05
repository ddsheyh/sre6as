CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    location VARCHAR(255),
    event_date TIMESTAMP NOT NULL,
    price NUMERIC(10,2) NOT NULL,
    available_tickets INT NOT NULL,
    image_url VARCHAR(500) DEFAULT '',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    event_id INT REFERENCES events(id),
    quantity INT NOT NULL DEFAULT 1,
    total_price NUMERIC(10,2) NOT NULL,
    status VARCHAR(50) DEFAULT 'confirmed',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    username VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO events (title, description, location, event_date, price, available_tickets) VALUES
('Music Festival 2026', 'Annual summer festival featuring top artists and bands from around the world. Three stages, food courts, and camping area.', 'Central Park, Qaragandy', '2026-07-15 18:00:00', 4500.00, 500),
('Tech Conference 2026', 'Leading technology conference covering AI, cloud computing, and DevOps. Networking opportunities and workshops included.', 'Expo Center, Astana', '2026-06-20 09:00:00', 7500.00, 300),
('Comedy Night Show', 'An evening of stand-up comedy with the best comedians. Guaranteed laughs and great atmosphere.', 'Comedy Club, Almaty', '2026-05-10 20:00:00', 2500.00, 150),
('Jazz Evening', 'Smooth jazz concert featuring local and international jazz musicians. Wine and appetizers available.', 'Philharmonic Hall, Almaty', '2026-08-05 19:30:00', 3500.00, 200),
('Football Match: Real Madrid vs Shakter', 'Premier League match between two top teams. Come support your favorite team!', 'Central Stadium, Almaty', '2026-05-25 17:00:00', 1500.00, 1000),
('Art Exhibition Opening', 'Contemporary art exhibition featuring works by emerging artists from Central Asia. Free welcome drink.', 'Modern Art Gallery, Astana', '2026-06-01 16:00:00', 1000.00, 250);
