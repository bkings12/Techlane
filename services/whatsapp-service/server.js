const express = require('express');
const cors = require('cors');
const crypto = require('crypto');
const {
    default: makeWASocket,
    DisconnectReason,
    useMultiFileAuthState,
    fetchLatestBaileysVersion,
    fetchLatestWaWebVersion,
    Browsers,
} = require('@whiskeysockets/baileys');
const QRCode = require('qrcode');
const pino = require('pino');
const fs = require('fs');
const path = require('path');

// Load backend/.env when running beside Nest (PM2 cwd = whatsapp-service)
try {
    require('dotenv').config({ path: path.join(__dirname, '..', '.env') });
} catch (_) {
    /* optional */
}

// Make crypto available globally for Baileys
if (!globalThis.crypto) {
    globalThis.crypto = crypto;
}

const http = require('http');
const https = require('https');

const app = express();
app.use(cors());
app.use(express.json());

const PORT = process.env.WHATSAPP_PORT || 3001;
const AUTH_DIR = path.join(__dirname, 'auth_sessions');
const CHATS_DIR = path.join(__dirname, 'chat_data');

// Store active sessions per tenant
const sessions = new Map();
const qrCodes = new Map();
const connectionStatus = new Map();
const chatStore = new Map(); // Store chats and messages per tenant
/** Prevent overlapping makeWASocket calls for the same tenant (QR races break Business pairing). */
const initLocks = new Map();
/** LID (local id) → phone digits, so inbound @lid replies map to customer MSISDNs. */
const lidPhoneMap = new Map(); // key: `${tenantId}:${lid}` → phone digits

function lidMapPath(tenantId) {
    return path.join(CHATS_DIR, `${tenantId || 'default'}_lid_map.json`);
}

function loadLidMap(tenantId) {
    const sessionId = tenantId || 'default';
    try {
        const filePath = lidMapPath(sessionId);
        if (fs.existsSync(filePath)) {
            const data = JSON.parse(fs.readFileSync(filePath, 'utf8'));
            for (const [lid, phone] of Object.entries(data || {})) {
                if (lid && phone) lidPhoneMap.set(`${sessionId}:${lid}`, String(phone));
            }
        }
    } catch (e) {
        console.error(`[${sessionId}] lid map load error:`, e.message);
    }
}

function saveLidMap(tenantId) {
    const sessionId = tenantId || 'default';
    const out = {};
    const prefix = `${sessionId}:`;
    for (const [k, v] of lidPhoneMap.entries()) {
        if (k.startsWith(prefix)) out[k.slice(prefix.length)] = v;
    }
    try {
        fs.writeFileSync(lidMapPath(sessionId), JSON.stringify(out, null, 2));
    } catch (e) {
        console.error(`[${sessionId}] lid map save error:`, e.message);
    }
}

function rememberLidPhone(tenantId, lidOrJid, phoneOrJid) {
    const sessionId = tenantId || 'default';
    const lid = String(lidOrJid || '').split('@')[0].replace(/\D/g, '');
    let phone = String(phoneOrJid || '').split('@')[0].replace(/\D/g, '');
    if (!lid || !phone) return;
    // Ignore if "phone" looks like a LID (very long) and lid is short — still store best-effort.
    if (phone.startsWith('0') && phone.length >= 10) phone = '254' + phone.slice(1);
    lidPhoneMap.set(`${sessionId}:${lid}`, phone);
    saveLidMap(sessionId);
}

function phoneFromJid(tenantId, jid, rawMsg) {
    const sessionId = tenantId || 'default';
    const key = rawMsg?.key || {};
    const candidates = [
        jid,
        key.remoteJid,
        key.remoteJidAlt,
        key.participantAlt,
        key.participant,
        rawMsg?.senderPn,
        key.senderPn,
    ].filter(Boolean);

    for (const c of candidates) {
        const s = String(c);
        if (s.includes('@s.whatsapp.net') || s.includes('@c.us')) {
            return s.split('@')[0].replace(/\D/g, '');
        }
    }
    for (const c of candidates) {
        const s = String(c);
        if (s.includes('@lid')) {
            const lid = s.split('@')[0].replace(/\D/g, '');
            const mapped = lidPhoneMap.get(`${sessionId}:${lid}`);
            if (mapped) return mapped;
        }
    }
    // Last resort: digits from jid (works for PN jids).
    const digits = String(jid || '').split('@')[0].replace(/\D/g, '');
    return digits || '';
}

function rememberMappingFromMessage(tenantId, jid, rawMsg) {
    const key = rawMsg?.key || {};
    const parts = [jid, key.remoteJid, key.remoteJidAlt, key.participant, key.participantAlt];
    const lid = parts.find((p) => p && String(p).includes('@lid'));
    const pn = parts.find((p) => p && (String(p).includes('@s.whatsapp.net') || String(p).includes('@c.us')));
    if (lid && pn) rememberLidPhone(tenantId, lid, pn);
}

// Logger
const logger = pino({ level: 'warn' });

// Ensure directories exist
if (!fs.existsSync(AUTH_DIR)) {
    fs.mkdirSync(AUTH_DIR, { recursive: true });
}
if (!fs.existsSync(CHATS_DIR)) {
    fs.mkdirSync(CHATS_DIR, { recursive: true });
}

// Get session directory for a tenant
function getSessionDir(tenantId) {
    const dir = path.join(AUTH_DIR, tenantId || 'default');
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
    return dir;
}

// Get chat storage file for a tenant
function getChatStorePath(tenantId) {
    return path.join(CHATS_DIR, `${tenantId || 'default'}_chats.json`);
}

// Load chats from disk
function loadChats(tenantId) {
    const filePath = getChatStorePath(tenantId);
    try {
        if (fs.existsSync(filePath)) {
            const data = JSON.parse(fs.readFileSync(filePath, 'utf8'));
            chatStore.set(tenantId || 'default', data);
            return data;
        }
    } catch (e) {
        console.error(`[${tenantId}] Error loading chats:`, e);
    }
    return { chats: {}, messages: {} };
}

// Save chats to disk
function saveChats(tenantId) {
    const filePath = getChatStorePath(tenantId);
    const data = chatStore.get(tenantId || 'default') || { chats: {}, messages: {} };
    try {
        fs.writeFileSync(filePath, JSON.stringify(data, null, 2));
    } catch (e) {
        console.error(`[${tenantId}] Error saving chats:`, e);
    }
}

// Store a message
function storeMessage(tenantId, jid, message, fromMe = false) {
    const sessionId = tenantId || 'default';
    if (!chatStore.has(sessionId)) {
        chatStore.set(sessionId, { chats: {}, messages: {} });
    }
    const store = chatStore.get(sessionId);
    
    // Update chat info
    const chatId = jid.split('@')[0];
    if (!store.chats[chatId]) {
        store.chats[chatId] = {
            id: chatId,
            jid: jid,
            name: message.pushName || chatId,
            lastMessage: '',
            lastMessageTime: Date.now(),
            unreadCount: 0
        };
    }
    
    // Extract message content
    let content = '';
    let type = 'text';
    if (message.message) {
        const msg = message.message;
        if (msg.conversation) {
            content = msg.conversation;
        } else if (msg.extendedTextMessage?.text) {
            content = msg.extendedTextMessage.text;
        } else if (msg.imageMessage) {
            content = msg.imageMessage.caption || '[Image]';
            type = 'image';
        } else if (msg.videoMessage) {
            content = msg.videoMessage.caption || '[Video]';
            type = 'video';
        } else if (msg.audioMessage) {
            content = '[Audio]';
            type = 'audio';
        } else if (msg.documentMessage) {
            content = msg.documentMessage.fileName || '[Document]';
            type = 'document';
        } else if (msg.stickerMessage) {
            content = '[Sticker]';
            type = 'sticker';
        } else if (msg.contactMessage) {
            content = '[Contact]';
            type = 'contact';
        } else if (msg.locationMessage) {
            content = '[Location]';
            type = 'location';
        } else if (msg.buttonsResponseMessage) {
            content = msg.buttonsResponseMessage.selectedDisplayText || '[Button Response]';
        } else if (msg.listResponseMessage) {
            content = msg.listResponseMessage.title || '[List Response]';
        } else if (msg.templateButtonReplyMessage) {
            content = msg.templateButtonReplyMessage.selectedDisplayText || '[Template Reply]';
        } else if (msg.protocolMessage) {
            // Skip protocol messages (receipts, etc.)
            return null;
        } else if (msg.reactionMessage) {
            content = msg.reactionMessage.text || '[Reaction]';
            type = 'reaction';
        } else {
            // Try to get any text content
            const keys = Object.keys(msg);
            for (const key of keys) {
                if (msg[key]?.text) {
                    content = msg[key].text;
                    break;
                } else if (msg[key]?.caption) {
                    content = msg[key].caption;
                    break;
                }
            }
            if (!content) {
                console.log('Unknown message type:', keys);
                content = '[Message]';
            }
        }
    }
    
    // Skip if no content (protocol messages, etc.)
    if (!content) {
        return null;
    }
    
    // Update chat
    store.chats[chatId].lastMessage = content;
    store.chats[chatId].lastMessageTime = message.messageTimestamp ? message.messageTimestamp * 1000 : Date.now();
    store.chats[chatId].name = message.pushName || store.chats[chatId].name || chatId;
    if (!fromMe) {
        store.chats[chatId].unreadCount = (store.chats[chatId].unreadCount || 0) + 1;
    }
    
    // Store message
    if (!store.messages[chatId]) {
        store.messages[chatId] = [];
    }
    
    const msgData = {
        id: message.key?.id || Date.now().toString(),
        content: content,
        type: type,
        fromMe: fromMe,
        timestamp: message.messageTimestamp ? message.messageTimestamp * 1000 : Date.now(),
        status: message.status || 1,
        pushName: message.pushName
    };
    
    // Avoid duplicates
    const exists = store.messages[chatId].find(m => m.id === msgData.id);
    if (!exists) {
        store.messages[chatId].push(msgData);
        // Keep only last 100 messages per chat
        if (store.messages[chatId].length > 100) {
            store.messages[chatId] = store.messages[chatId].slice(-100);
        }
    }
    
    saveChats(sessionId);
    return msgData;
}

/** Default Nest inbound URL on same host (override with WHATSAPP_INBOUND_URL in backend/.env). */
function defaultInboundUrl() {
    const p = process.env.BACKEND_HTTP_PORT || process.env.NEST_PORT || '4000';
    return `http://127.0.0.1:${p}/api/whatsapp/inbound`;
}

let inboundUrlLoggedOnce = false;

/**
 * Notify Nest backend for AI auto-replies. Uses WHATSAPP_INBOUND_URL or defaults to http://127.0.0.1:PORT/api/whatsapp/inbound
 */
function notifyNestInbound(sessionId, jid, msgData, rawMsg, fromMe) {
    if (fromMe) return;
    const text = msgData?.content;
    if (!text || typeof text !== 'string') return;
    if (text.startsWith('[')) return;
    const base = (process.env.WHATSAPP_INBOUND_URL || '').trim() || defaultInboundUrl();
    if (!inboundUrlLoggedOnce) {
        inboundUrlLoggedOnce = true;
        const fromEnv = !!(process.env.WHATSAPP_INBOUND_URL || '').trim();
        console.log(
            `[whatsapp-service] AI inbound → ${base}${fromEnv ? ' (WHATSAPP_INBOUND_URL)' : ' (default; set WHATSAPP_INBOUND_URL if Nest uses another host/port)'}`,
        );
    }
    const secret = process.env.WHATSAPP_SERVICE_SECRET || '';
    if (!secret.trim()) {
        console.error('[whatsapp-service] WHATSAPP_SERVICE_SECRET missing — cannot notify Nest for AI');
        return;
    }
    rememberMappingFromMessage(sessionId, jid, rawMsg);
    const phone = phoneFromJid(sessionId, jid, rawMsg);
    if (!phone) {
        console.error(`[${sessionId}] inbound skip — could not resolve phone from jid=${jid} alt=${rawMsg?.key?.remoteJidAlt || ''}`);
        return;
    }
    const urlStr = base.replace(/\/$/, '');
    const payload = JSON.stringify({
        tenantId: sessionId,
        phone,
        text,
        pushName: rawMsg?.pushName || undefined,
        jid: String(jid || ''),
    });
    console.log(`[${sessionId}] inbound → platform phone=${phone} text=${JSON.stringify(text).slice(0, 80)}`);
    try {
        const u = new URL(urlStr);
        const isHttps = u.protocol === 'https:';
        const mod = isHttps ? https : http;
        const port = u.port || (isHttps ? 443 : 80);
        const options = {
            hostname: u.hostname,
            port,
            path: u.pathname + u.search,
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(payload),
                'X-WhatsApp-Service-Key': secret,
            },
            timeout: 25_000,
        };
        const req = mod.request(options, (res) => {
            let body = '';
            res.on('data', (c) => { body += c; });
            res.on('end', () => {
                if (res.statusCode && res.statusCode >= 400) {
                    console.error(`[whatsapp-service] inbound POST failed: HTTP ${res.statusCode} → ${base} ${body.slice(0, 200)}`);
                } else {
                    console.log(`[${sessionId}] inbound OK HTTP ${res.statusCode}`);
                }
            });
        });
        req.on('error', (err) => {
            console.error(`[whatsapp-service] inbound network error → ${base}:`, err.message);
        });
        req.write(payload);
        req.end();
    } catch (e) {
        console.error('notifyNestInbound error:', e.message);
    }
}

// Sync existing chats after connection
async function syncExistingChats(tenantId, sock) {
    const sessionId = tenantId || 'default';
    console.log(`[${sessionId}] Starting chat sync...`);
    
    try {
        // Get contacts/chats from the socket store if available
        if (sock.store) {
            const chats = sock.store.chats?.all?.() || [];
            console.log(`[${sessionId}] Found ${chats.length} chats in store`);
            
            for (const chat of chats) {
                if (chat.id && !chat.id.includes('@g.us') && !chat.id.includes('@broadcast')) {
                    const chatId = chat.id.split('@')[0];
                    if (!chatStore.has(sessionId)) {
                        chatStore.set(sessionId, { chats: {}, messages: {} });
                    }
                    const store = chatStore.get(sessionId);
                    if (!store.chats[chatId]) {
                        store.chats[chatId] = {
                            id: chatId,
                            jid: chat.id,
                            name: chat.name || chat.notify || chatId,
                            lastMessage: '',
                            lastMessageTime: Date.now(),
                            unreadCount: chat.unreadCount || 0
                        };
                    }
                }
            }
            saveChats(sessionId);
        }
    } catch (e) {
        console.error(`[${sessionId}] Error syncing chats:`, e);
    }
}

async function resolveWaVersion() {
    // Live WhatsApp Web revision pairs more reliably (esp. WhatsApp Business).
    try {
        const live = await fetchLatestWaWebVersion();
        if (live?.version?.length) {
            console.log(`[wa] using live web version ${live.version.join('.')}`);
            return live.version;
        }
    } catch (err) {
        console.warn('[wa] fetchLatestWaWebVersion failed:', err?.message || err);
    }
    const fallback = await fetchLatestBaileysVersion();
    console.log(`[wa] using baileys version ${(fallback.version || []).join('.')}`);
    return fallback.version;
}

// Initialize WhatsApp connection for a tenant
async function initializeSession(tenantId) {
    const sessionId = tenantId || 'default';

    if (initLocks.get(sessionId)) {
        return initLocks.get(sessionId);
    }

    const run = (async () => {
    // Load existing chats + LID map
    loadChats(sessionId);
    loadLidMap(sessionId);
    
    // Check if already connected
    if (sessions.has(sessionId)) {
        const sock = sessions.get(sessionId);
        if (sock && sock.user && connectionStatus.get(sessionId) === 'connected') {
            return { status: 'connected', user: sock.user };
        }
        // Drop stale half-open sockets before opening a new one.
        try {
            sock?.end?.(undefined);
        } catch (_) {
            /* ignore */
        }
        sessions.delete(sessionId);
    }

    const sessionDir = getSessionDir(sessionId);
    const { state, saveCreds } = await useMultiFileAuthState(sessionDir);
    const version = await resolveWaVersion();

    // macOS Chrome fingerprint — custom names like "TechLane" are rejected by
    // WhatsApp Business pairing more often than personal WhatsApp.
    const sock = makeWASocket({
        version,
        auth: state,
        printQRInTerminal: false,
        logger,
        browser: Browsers.macOS('Chrome'),
        syncFullHistory: false,
        markOnlineOnConnect: false,
        connectTimeoutMs: 60_000,
        defaultQueryTimeoutMs: 60_000,
        qrTimeout: 90_000,
        getMessage: async () => ({ conversation: '' }),
    });

    // Handle connection updates
    sock.ev.on('connection.update', async (update) => {
        // Ignore events from a replaced socket.
        if (sessions.get(sessionId) !== sock) return;

        const { connection, lastDisconnect, qr } = update;

        if (qr) {
            // Generate QR code as base64
            try {
                const qrDataUrl = await QRCode.toDataURL(qr, {
                    width: 512,
                    margin: 2,
                    errorCorrectionLevel: 'M',
                    color: { dark: '#000000', light: '#ffffff' },
                });
                qrCodes.set(sessionId, qrDataUrl);
                connectionStatus.set(sessionId, 'waiting_qr');
                console.log(`[${sessionId}] QR Code generated`);
            } catch (err) {
                console.error(`[${sessionId}] QR generation error:`, err);
            }
        }

        if (connection === 'close') {
            const statusCode = lastDisconnect?.error?.output?.statusCode;
            const loggedOut = statusCode === DisconnectReason.loggedOut;
            // 515 = restart required after QR scan — keep auth, reopen socket.
            const restartRequired = statusCode === DisconnectReason.restartRequired || statusCode === 515;
            
            console.log(`[${sessionId}] Connection closed, status: ${statusCode}`);
            if (sessions.get(sessionId) === sock) {
                sessions.delete(sessionId);
            }
            connectionStatus.set(sessionId, loggedOut ? 'logged_out' : 'disconnected');
            if (!restartRequired) {
                qrCodes.delete(sessionId);
            }
            
            if (!loggedOut) {
                const delayMs = restartRequired ? 1500 : 5000;
                console.log(`[${sessionId}] Reconnecting in ${delayMs}ms...`);
                setTimeout(() => initializeSession(sessionId), delayMs);
            } else {
                // Clear auth files on logout
                const dir = getSessionDir(sessionId);
                fs.rmSync(dir, { recursive: true, force: true });
            }
        } else if (connection === 'open') {
            console.log(`[${sessionId}] Connected!`);
            connectionStatus.set(sessionId, 'connected');
            qrCodes.delete(sessionId);
            
            // Sync existing chats after connection
            syncExistingChats(sessionId, sock);
        }
    });

    // Handle chat updates (sync existing chats)
    sock.ev.on('chats.set', async ({ chats }) => {
        console.log(`[${sessionId}] Syncing ${chats.length} chats`);
        for (const chat of chats) {
            if (chat.id && !chat.id.includes('@g.us') && !chat.id.includes('@broadcast')) {
                const chatId = chat.id.split('@')[0];
                if (!chatStore.has(sessionId)) {
                    chatStore.set(sessionId, { chats: {}, messages: {} });
                }
                const store = chatStore.get(sessionId);
                if (!store.chats[chatId]) {
                    store.chats[chatId] = {
                        id: chatId,
                        jid: chat.id,
                        name: chat.name || chat.notify || chatId,
                        lastMessage: chat.lastMessage?.message?.conversation || '',
                        lastMessageTime: (chat.conversationTimestamp || 0) * 1000 || Date.now(),
                        unreadCount: chat.unreadCount || 0
                    };
                } else {
                    // Update existing chat
                    store.chats[chatId].name = chat.name || chat.notify || store.chats[chatId].name;
                    store.chats[chatId].unreadCount = chat.unreadCount || store.chats[chatId].unreadCount;
                }
            }
        }
        saveChats(sessionId);
    });

    // Handle chat upserts
    sock.ev.on('chats.upsert', async (chats) => {
        console.log(`[${sessionId}] Chat upsert: ${chats.length} chats`);
        for (const chat of chats) {
            if (chat.id && !chat.id.includes('@g.us') && !chat.id.includes('@broadcast')) {
                const chatId = chat.id.split('@')[0];
                if (!chatStore.has(sessionId)) {
                    chatStore.set(sessionId, { chats: {}, messages: {} });
                }
                const store = chatStore.get(sessionId);
                if (!store.chats[chatId]) {
                    store.chats[chatId] = {
                        id: chatId,
                        jid: chat.id,
                        name: chat.name || chat.notify || chatId,
                        lastMessage: '',
                        lastMessageTime: (chat.conversationTimestamp || 0) * 1000 || Date.now(),
                        unreadCount: chat.unreadCount || 0
                    };
                }
            }
        }
        saveChats(sessionId);
    });

    // Handle contacts updates
    sock.ev.on('contacts.upsert', async (contacts) => {
        console.log(`[${sessionId}] Contacts upsert: ${contacts.length} contacts`);
        for (const contact of contacts) {
            if (contact.id && !contact.id.includes('@g.us') && !contact.id.includes('@broadcast')) {
                const chatId = contact.id.split('@')[0];
                if (!chatStore.has(sessionId)) {
                    chatStore.set(sessionId, { chats: {}, messages: {} });
                }
                const store = chatStore.get(sessionId);
                if (store.chats[chatId]) {
                    store.chats[chatId].name = contact.name || contact.notify || store.chats[chatId].name;
                }
            }
        }
        saveChats(sessionId);
    });

    // Handle incoming messages
    sock.ev.on('messages.upsert', async (m) => {
        console.log(`[${sessionId}] Messages upsert: ${m.messages.length} messages, type: ${m.type}`);
        for (const msg of m.messages) {
            const jid = msg.key.remoteJid;
            if (jid && !jid.includes('@g.us') && !jid.includes('@broadcast')) {
                const fromMe = msg.key.fromMe || false;
                rememberMappingFromMessage(sessionId, jid, msg);
                const hasBody = !!(msg.message && (msg.message.conversation || msg.message.extendedTextMessage || msg.message.buttonsResponseMessage));
                console.log(
                    `[${sessionId}] ${fromMe ? 'Sent' : 'Received'} message ${fromMe ? 'to' : 'from'} ${jid}` +
                    ` alt=${msg.key.remoteJidAlt || '-'} body=${hasBody}`,
                );
                const stored = storeMessage(sessionId, jid, msg, fromMe);
                if (!fromMe && !stored) {
                    console.warn(`[${sessionId}] inbound stored=null (empty/undecryptable?) jid=${jid}`);
                }
                if (!fromMe && stored) {
                    notifyNestInbound(sessionId, jid, stored, msg, fromMe);
                }
            }
        }
    });

    // Persist PN↔LID mappings when Baileys discovers them.
    if (sock.ev.on) {
        sock.ev.on('lid-mapping.update', (map) => {
            try {
                const entries = map && typeof map === 'object' ? Object.entries(map) : [];
                for (const [a, b] of entries) {
                    if (String(a).includes('@lid') || /^\d+$/.test(String(a))) {
                        rememberLidPhone(sessionId, a, b);
                    } else {
                        rememberLidPhone(sessionId, b, a);
                    }
                }
                console.log(`[${sessionId}] lid-mapping.update (${entries.length})`);
            } catch (e) {
                console.error(`[${sessionId}] lid-mapping.update error:`, e.message);
            }
        });
    }
    
    // Handle message updates (read receipts, delivery status)
    sock.ev.on('messages.update', async (updates) => {
        for (const update of updates) {
            if (update.update?.status) {
                const jid = update.key?.remoteJid;
                const msgId = update.key?.id;
                if (jid && msgId) {
                    const chatId = jid.split('@')[0];
                    const store = chatStore.get(sessionId);
                    if (store?.messages[chatId]) {
                        const msg = store.messages[chatId].find(m => m.id === msgId);
                        if (msg) {
                            msg.status = update.update.status;
                            saveChats(sessionId);
                        }
                    }
                }
            }
        }
    });

    // Save credentials on update
    sock.ev.on('creds.update', saveCreds);

    sessions.set(sessionId, sock);
    return { status: 'initializing' };
    })();

    initLocks.set(sessionId, run);
    try {
        return await run;
    } finally {
        if (initLocks.get(sessionId) === run) {
            initLocks.delete(sessionId);
        }
    }
}

// API Routes

/** All routes below require X-WhatsApp-Service-Key matching WHATSAPP_SERVICE_SECRET (set by Nest proxy). */
function requireServiceSecret(req, res, next) {
    const secret = process.env.WHATSAPP_SERVICE_SECRET;
    if (!secret || !String(secret).trim()) {
        return res.status(503).json({ success: false, error: 'WHATSAPP_SERVICE_SECRET is not set on WhatsApp service' });
    }
    const key = req.headers['x-whatsapp-service-key'];
    if (key !== secret) {
        return res.status(401).json({ success: false, error: 'Unauthorized' });
    }
    next();
}

// Health check (no auth — use only on internal network / probes)
app.get('/health', (req, res) => {
    res.json({ status: 'ok', sessions: sessions.size });
});

app.use(requireServiceSecret);

// Get QR code for linking
app.get('/qr/:tenantId?', async (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    
    // Initialize session if not exists
    if (!sessions.has(tenantId)) {
        await initializeSession(tenantId);
    }

    // Wait a bit for QR to generate
    let attempts = 0;
    while (!qrCodes.has(tenantId) && attempts < 10) {
        await new Promise(r => setTimeout(r, 500));
        attempts++;
    }

    const qrCode = qrCodes.get(tenantId);
    const status = connectionStatus.get(tenantId) || 'unknown';

    if (status === 'connected') {
        const sock = sessions.get(tenantId);
        return res.json({
            success: true,
            status: 'connected',
            user: sock?.user || null,
            qr: null
        });
    }

    res.json({
        success: true,
        status,
        qr: qrCode || null,
        message: qrCode
            ? 'Scan with WhatsApp or WhatsApp Business → Linked devices'
            : 'Generating QR code...'
    });
});

// Check connection status
app.get('/status/:tenantId?', (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    const status = connectionStatus.get(tenantId) || 'not_initialized';
    const sock = sessions.get(tenantId);

    res.json({
        success: true,
        status,
        connected: status === 'connected',
        user: sock?.user || null,
        hasQr: qrCodes.has(tenantId)
    });
});

// Send message
app.post('/send', async (req, res) => {
    const { tenantId, phone, message, type = 'text' } = req.body;
    const sessionId = tenantId || 'default';

    if (!phone || !message) {
        return res.status(400).json({ success: false, error: 'Phone and message required' });
    }

    const sock = sessions.get(sessionId);
    if (!sock || connectionStatus.get(sessionId) !== 'connected') {
        return res.status(400).json({ success: false, error: 'WhatsApp not connected' });
    }

    try {
        let formattedPhone = phone;
        
        // Check if this is a stored chat ID (might be a LID format)
        const store = chatStore.get(sessionId) || loadChats(sessionId);
        const chat = store.chats[phone];
        if (chat && chat.jid) {
            // Use the stored JID directly
            formattedPhone = chat.jid;
        } else {
            // Format phone number (ensure it has country code)
            formattedPhone = phone.replace(/\D/g, '');
            if (formattedPhone.startsWith('0')) {
                formattedPhone = '254' + formattedPhone.substring(1); // Kenya
            }
            if (!formattedPhone.includes('@')) {
                formattedPhone = formattedPhone + '@s.whatsapp.net';
            }

            // Check if number exists on WhatsApp (only for new phone numbers)
            try {
                const [exists] = await sock.onWhatsApp(formattedPhone.split('@')[0]);
                if (!exists || exists.exists === false) {
                    // older baileys returns {jid, exists}; newer may differ
                    if (exists && exists.exists === false) {
                        return res.json({
                            success: false,
                            error: 'Number not registered on WhatsApp',
                            phone: formattedPhone
                        });
                    }
                    if (!exists) {
                        return res.json({
                            success: false,
                            error: 'Number not registered on WhatsApp',
                            phone: formattedPhone
                        });
                    }
                }
                if (exists?.lid) {
                    rememberLidPhone(sessionId, exists.lid, formattedPhone);
                }
                if (exists?.jid && String(exists.jid).includes('@lid')) {
                    rememberLidPhone(sessionId, exists.jid, formattedPhone);
                }
            } catch (checkErr) {
                console.error('Error checking WhatsApp existence:', checkErr.message);
                // Continue anyway - might be a LID or other format
            }
        }

        // Send message
        const result = await sock.sendMessage(formattedPhone, { text: message });
        
        // Store the sent message
        const sentMsg = {
            key: result.key,
            message: { conversation: message },
            messageTimestamp: Math.floor(Date.now() / 1000),
            pushName: sock.user?.name || 'Me'
        };
        storeMessage(sessionId, formattedPhone, sentMsg, true);

        res.json({
            success: true,
            messageId: result.key.id,
            phone: formattedPhone,
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        console.error(`[${sessionId}] Send error:`, error);
        res.status(500).json({ success: false, error: error.message });
    }
});

// Get all chats
app.get('/chats/:tenantId?', (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    const store = chatStore.get(tenantId) || loadChats(tenantId);
    
    // Convert to sorted array
    const chats = Object.values(store.chats || {}).sort((a, b) => b.lastMessageTime - a.lastMessageTime);
    
    res.json({
        success: true,
        chats: chats
    });
});

// Get messages for a specific chat
app.get('/messages/:chatId/:tenantId?', (req, res) => {
    const chatId = req.params.chatId;
    const tenantId = req.params.tenantId || req.query.tenantId || 'default';
    const store = chatStore.get(tenantId) || loadChats(tenantId);
    
    const messages = store.messages[chatId] || [];
    
    // Sort messages by timestamp
    messages.sort((a, b) => a.timestamp - b.timestamp);
    
    res.json({
        success: true,
        messages: messages,
        chat: store.chats[chatId] || null
    });
});

// Mark chat as read
app.post('/read/:chatId/:tenantId?', async (req, res) => {
    const chatId = req.params.chatId;
    const tenantId = req.params.tenantId || req.body.tenantId || 'default';
    
    const store = chatStore.get(tenantId) || loadChats(tenantId);
    if (store?.chats[chatId]) {
        store.chats[chatId].unreadCount = 0;
        saveChats(tenantId);
        
        // Also mark as read on WhatsApp
        const sock = sessions.get(tenantId);
        if (sock) {
            try {
                const jid = chatId + '@s.whatsapp.net';
                await sock.readMessages([{ remoteJid: jid, id: 'all' }]);
            } catch (e) {
                console.log(`[${tenantId}] Could not mark messages as read on WhatsApp:`, e.message);
            }
        }
    }
    
    res.json({ success: true });
});

// Get total unread count
app.get('/unread/:tenantId?', (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    const store = chatStore.get(tenantId) || loadChats(tenantId);
    
    let totalUnread = 0;
    Object.values(store.chats || {}).forEach(chat => {
        totalUnread += chat.unreadCount || 0;
    });
    
    res.json({
        success: true,
        unreadCount: totalUnread
    });
});

// Start a new chat (check if number exists)
app.post('/new-chat', async (req, res) => {
    const { tenantId, phone } = req.body;
    const sessionId = tenantId || 'default';

    if (!phone) {
        return res.status(400).json({ success: false, error: 'Phone number required' });
    }

    const sock = sessions.get(sessionId);
    if (!sock || connectionStatus.get(sessionId) !== 'connected') {
        return res.status(400).json({ success: false, error: 'WhatsApp not connected' });
    }

    try {
        // Format phone number
        let formattedPhone = phone.replace(/\D/g, '');
        if (formattedPhone.startsWith('0')) {
            formattedPhone = '254' + formattedPhone.substring(1);
        }

        // Check if number exists on WhatsApp
        const [exists] = await sock.onWhatsApp(formattedPhone);
        if (!exists) {
            return res.json({
                success: false,
                error: 'Number not registered on WhatsApp',
                phone: formattedPhone
            });
        }

        const jid = formattedPhone + '@s.whatsapp.net';
        const chatId = formattedPhone;

        // Create chat entry
        if (!chatStore.has(sessionId)) {
            chatStore.set(sessionId, { chats: {}, messages: {} });
        }
        const store = chatStore.get(sessionId);
        
        if (!store.chats[chatId]) {
            store.chats[chatId] = {
                id: chatId,
                jid: jid,
                name: exists.notify || chatId,
                lastMessage: '',
                lastMessageTime: Date.now(),
                unreadCount: 0
            };
            saveChats(sessionId);
        }

        res.json({
            success: true,
            chat: store.chats[chatId]
        });
    } catch (error) {
        console.error(`[${sessionId}] New chat error:`, error);
        res.status(500).json({ success: false, error: error.message });
    }
});

// Sync chats manually
app.post('/sync/:tenantId?', async (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    const sock = sessions.get(tenantId);

    if (!sock || connectionStatus.get(tenantId) !== 'connected') {
        return res.status(400).json({ success: false, error: 'WhatsApp not connected' });
    }

    try {
        await syncExistingChats(tenantId, sock);
        const store = chatStore.get(tenantId) || { chats: {}, messages: {} };
        const chats = Object.values(store.chats || {}).sort((a, b) => b.lastMessageTime - a.lastMessageTime);
        
        res.json({
            success: true,
            message: 'Sync complete',
            chatCount: chats.length,
            chats: chats
        });
    } catch (error) {
        console.error(`[${tenantId}] Sync error:`, error);
        res.status(500).json({ success: false, error: error.message });
    }
});

// Disconnect/Logout
app.post('/disconnect/:tenantId?', async (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    const sock = sessions.get(tenantId);

    if (sock) {
        await sock.logout();
        sessions.delete(tenantId);
        connectionStatus.delete(tenantId);
        qrCodes.delete(tenantId);
        
        // Clear auth files
        const dir = getSessionDir(tenantId);
        fs.rmSync(dir, { recursive: true, force: true });
    }

    res.json({ success: true, message: 'Disconnected' });
});

// Reconnect
app.post('/reconnect/:tenantId?', async (req, res) => {
    const tenantId = req.params.tenantId || 'default';
    
    // Close existing if any
    const existingSock = sessions.get(tenantId);
    if (existingSock) {
        try {
            existingSock.end();
        } catch (e) {}
        sessions.delete(tenantId);
    }

    await initializeSession(tenantId);
    res.json({ success: true, message: 'Reconnecting...' });
});

// Start server
app.listen(PORT, () => {
    console.log(`WhatsApp service running on port ${PORT}`);
    
    // Auto-initialize all existing sessions
    if (fs.existsSync(AUTH_DIR)) {
        const sessionDirs = fs.readdirSync(AUTH_DIR);
        for (const sessionId of sessionDirs) {
            const sessionDir = path.join(AUTH_DIR, sessionId);
            if (fs.statSync(sessionDir).isDirectory() && fs.existsSync(path.join(sessionDir, 'creds.json'))) {
                console.log(`Found existing session: ${sessionId}, initializing...`);
                initializeSession(sessionId);
            }
        }
    }
});
