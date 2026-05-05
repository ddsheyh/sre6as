const API = '';  // same origin via nginx proxy
let token = localStorage.getItem('token');
let userName = localStorage.getItem('userName');

const icons = ['😂', '⚽', '🎨', '💻', '🎸', '🎷'];

function navigate(page, data) {
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById('page-' + page).classList.add('active');
    if (page === 'home') loadEvents();
    if (page === 'event') loadEventDetail(data);
    if (page === 'profile') loadProfile();
    if (page === 'chat') loadMessages();
}

function updateNav() {
    const loggedIn = !!token;
    document.getElementById('nav-login').style.display = loggedIn ? 'none' : 'block';
    document.getElementById('nav-logout').style.display = loggedIn ? 'block' : 'none';
    document.getElementById('nav-profile').style.display = loggedIn ? 'block' : 'none';
    document.getElementById('nav-chat').style.display = loggedIn ? 'block' : 'none';
}

function logout() {
    token = null; userName = null;
    localStorage.removeItem('token');
    localStorage.removeItem('userName');
    updateNav();
    navigate('home');
    showToast('Logged out', 'success');
}

async function api(method, path, body) {
    const opts = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (token) opts.headers['Authorization'] = 'Bearer ' + token;
    if (body) opts.body = JSON.stringify(body);
    const res = await fetch(API + path, opts);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    return data;
}

function showToast(msg, type) {
    const t = document.getElementById('toast');
    t.textContent = msg;
    t.className = 'toast ' + (type || '') + ' show';
    setTimeout(() => t.classList.remove('show'), 3000);
}

function formatDate(dateStr) {
    try {
        const d = new Date(dateStr);
        return d.toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    } catch { return dateStr; }
}

function formatPrice(price) {
    return Number(price).toLocaleString('en-US', { minimumFractionDigits: 0 }) + ' ₸';
}

async function handleLogin(e) {
    e.preventDefault();
    const errEl = document.getElementById('login-error');
    errEl.textContent = '';
    try {
        const data = await api('POST', '/api/auth/login', {
            email: document.getElementById('login-email').value,
            password: document.getElementById('login-password').value
        });
        token = data.token;
        userName = data.name;
        localStorage.setItem('token', token);
        localStorage.setItem('userName', userName);
        updateNav();
        navigate('home');
        showToast('Welcome back, ' + data.name + '!', 'success');
    } catch (err) {
        errEl.textContent = err.message;
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const errEl = document.getElementById('register-error');
    errEl.textContent = '';
    try {
        const data = await api('POST', '/api/auth/register', {
            name: document.getElementById('reg-name').value,
            email: document.getElementById('reg-email').value,
            password: document.getElementById('reg-password').value
        });
        token = data.token;
        userName = data.name;
        localStorage.setItem('token', token);
        localStorage.setItem('userName', userName);
        updateNav();
        navigate('home');
        showToast('Account created! Welcome!', 'success');
    } catch (err) {
        errEl.textContent = err.message;
    }
}

async function loadEvents() {
    const grid = document.getElementById('events-grid');
    grid.innerHTML = '<div class="loading">Loading events...</div>';
    try {
        const events = await api('GET', '/api/events');
        if (!events || events.length === 0) {
            grid.innerHTML = '<div class="empty-state">No events available</div>';
            return;
        }
        grid.innerHTML = events.map((ev, i) => `
            <div class="event-card" onclick="navigate('event', ${ev.id})" id="event-card-${ev.id}">
                <div class="event-card-img">${icons[i % icons.length]}</div>
                <div class="event-card-body">
                    <h3>${ev.title}</h3>
                    <div class="event-meta">
                        <span>📅 ${formatDate(ev.event_date)}</span>
                        <span>📍 ${ev.location}</span>
                        <span>🎟️ ${ev.available_tickets} tickets left</span>
                    </div>
                    <div class="event-price">${formatPrice(ev.price)}</div>
                </div>
            </div>
        `).join('');
    } catch (err) {
        grid.innerHTML = '<div class="empty-state">Failed to load events: ' + err.message + '</div>';
    }
}

async function loadEventDetail(eventId) {
    const el = document.getElementById('event-detail');
    el.innerHTML = '<div class="loading">Loading...</div>';
    try {
        const ev = await api('GET', '/api/events/' + eventId);
        const idx = ev.id - 1;
        el.innerHTML = `
            <a href="#" class="back-link" onclick="navigate('home')">← Back to Events</a>
            <div class="event-detail-card">
                <h2>${icons[idx % icons.length]} ${ev.title}</h2>
                <div class="meta-row">
                    <span>📅 ${formatDate(ev.event_date)}</span>
                    <span>📍 ${ev.location}</span>
                </div>
                <div class="meta-row">
                    <span>🎟️ ${ev.available_tickets} tickets available</span>
                </div>
                <div class="description">${ev.description}</div>
                <div class="price-section">
                    <div>
                        <div class="price-big">${formatPrice(ev.price)}</div>
                        <div class="qty-control">
                            <label>Qty:</label>
                            <input type="number" id="ticket-qty" value="1" min="1" max="${ev.available_tickets}">
                        </div>
                    </div>
                    <button class="btn btn-success" onclick="buyTicket(${ev.id})" id="buy-btn">
                        Buy Ticket
                    </button>
                </div>
            </div>
        `;
    } catch (err) {
        el.innerHTML = '<div class="empty-state">Event not found</div>';
    }
}

async function buyTicket(eventId) {
    if (!token) {
        showToast('Please login first', 'error');
        navigate('login');
        return;
    }
    const qty = parseInt(document.getElementById('ticket-qty').value) || 1;
    try {
        const data = await api('POST', '/api/orders', { event_id: eventId, quantity: qty });
        showToast('Order confirmed! Total: ' + formatPrice(data.total_price), 'success');
        navigate('profile');
    } catch (err) {
        showToast('Order failed: ' + err.message, 'error');
    }
}

async function loadProfile() {
    if (!token) { navigate('login'); return; }
    try {
        const user = await api('GET', '/api/users/me');
        document.getElementById('profile-info').innerHTML = `
            <p><strong>Name:</strong> ${user.name}</p>
            <p><strong>Email:</strong> ${user.email}</p>
            <p><strong>Member since:</strong> ${formatDate(user.created_at)}</p>
        `;
    } catch {
        document.getElementById('profile-info').innerHTML = '<p>Failed to load profile</p>';
    }
    try {
        const orders = await api('GET', '/api/orders');
        const list = document.getElementById('orders-list');
        if (!orders || orders.length === 0) {
            list.innerHTML = '<div class="empty-state">No orders yet. Browse events and buy tickets!</div>';
            return;
        }
        list.innerHTML = orders.map(o => `
            <div class="order-card" id="order-${o.id}">
                <div class="order-info">
                    <h4>${o.event_title}</h4>
                    <p>Qty: ${o.quantity} · Total: ${formatPrice(o.total_price)} · ${formatDate(o.created_at)}</p>
                </div>
                <span class="order-status">${o.status}</span>
            </div>
        `).join('');
    } catch {
        document.getElementById('orders-list').innerHTML = '<div class="empty-state">Failed to load orders</div>';
    }
}

async function loadMessages() {
    if (!token) { navigate('login'); return; }
    const el = document.getElementById('chat-messages');
    try {
        const msgs = await api('GET', '/api/messages');
        if (!msgs || msgs.length === 0) {
            el.innerHTML = '<div class="empty-state">No messages yet. Start the conversation!</div>';
            return;
        }
        el.innerHTML = msgs.reverse().map(m => `
            <div class="chat-msg">
                <div class="msg-user">${m.username}</div>
                <div class="msg-text">${m.content}</div>
                <div class="msg-time">${formatDate(m.created_at)}</div>
            </div>
        `).join('');
        el.scrollTop = el.scrollHeight;
    } catch {
        el.innerHTML = '<div class="empty-state">Failed to load messages</div>';
    }
}

async function sendMessage(e) {
    e.preventDefault();
    const input = document.getElementById('chat-input');
    const content = input.value.trim();
    if (!content) return;
    try {
        await api('POST', '/api/messages', { content });
        input.value = '';
        loadMessages();
    } catch (err) {
        showToast('Failed to send: ' + err.message, 'error');
    }
}

updateNav();
navigate('home');
