import { create } from 'zustand';
import axios from 'axios';

const API_BASE = import.meta.env.VITE_API_URL || '/api';

export const api = axios.create({ baseURL: API_BASE });

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// ==================== AUTH STORE ====================
export const useAuthStore = create((set) => ({
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  token: localStorage.getItem('token'),
  isAuthenticated: !!localStorage.getItem('token'),

  login: async (username, password) => {
    const { data } = await api.post('/auth/login', { username, password });
    localStorage.setItem('token', data.token);
    localStorage.setItem('user', JSON.stringify(data.user));
    set({ user: data.user, token: data.token, isAuthenticated: true });
    return data;
  },

  register: async (username, password) => {
    const { data } = await api.post('/auth/register', { username, password });
    localStorage.setItem('token', data.token);
    localStorage.setItem('user', JSON.stringify(data.user));
    set({ user: data.user, token: data.token, isAuthenticated: true });
    return data;
  },

  logout: () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    localStorage.removeItem('currentWorkspace');
    set({ user: null, token: null, isAuthenticated: false });
  },

  fetchUsers: async () => {
    const { data } = await api.get('/users');
    return data.users || [];
  },
}));

// ==================== WORKSPACE STORE ====================
export const useWorkspaceStore = create((set, get) => ({
  workspaces: [],
  current: JSON.parse(localStorage.getItem('currentWorkspace') || 'null'),
  members: [],
  roles: [],
  folders: [],
  loading: false,

  fetchWorkspaces: async () => {
    set({ loading: true });
    try {
      const { data } = await api.get('/workspaces');
      set({ workspaces: data.workspaces || [], loading: false });
    } catch {
      set({ loading: false });
    }
  },

  selectWorkspace: (ws) => {
    localStorage.setItem('currentWorkspace', JSON.stringify(ws));
    set({ current: ws });
  },

  createWorkspace: async (name) => {
    const { data } = await api.post('/workspaces', { name });
    await get().fetchWorkspaces();
    return data;
  },

  fetchMembers: async (wsId) => {
    const { data } = await api.get(`/workspaces/${wsId}/members`);
    set({ members: data.members || [] });
  },

  inviteMember: async (wsId, ngacNodeId) => {
    await api.post(`/workspaces/${wsId}/invite`, { ngac_node_id: ngacNodeId });
    await get().fetchMembers(wsId);
  },

  removeMember: async (wsId, nodeId) => {
    await api.delete(`/workspaces/${wsId}/members/${nodeId}`);
    await get().fetchMembers(wsId);
  },

  fetchRoles: async (wsId) => {
    const { data } = await api.get(`/workspaces/${wsId}/roles`);
    set({ roles: data.roles || [] });
  },

  createRole: async (wsId, name) => {
    await api.post(`/workspaces/${wsId}/roles`, { name });
    await get().fetchRoles(wsId);
  },

  fetchFolders: async (wsId) => {
    const { data } = await api.get(`/workspaces/${wsId}/folders`);
    set({ folders: data.folders || [] });
  },

  createFolder: async (wsId, name, parentOaId) => {
    await api.post(`/workspaces/${wsId}/folders`, { name, parent_oa_id: parentOaId });
    await get().fetchFolders(wsId);
  },

  createPermission: async (wsId, uaId, oaId, operations) => {
    await api.post(`/workspaces/${wsId}/permissions`, { ua_id: uaId, oa_id: oaId, operations });
  },
}));

// ==================== DOCUMENT STORE ====================
export const useDocStore = create((set, get) => ({
  documents: [],
  loading: false,
  error: null,

  fetchDocuments: async (wsId) => {
    set({ loading: true, error: null });
    try {
      const { data } = await api.get(`/workspaces/${wsId}/documents`);
      set({ documents: data.documents || [], loading: false });
    } catch (err) {
      set({ error: err.response?.data?.error || 'Failed to load documents', loading: false });
    }
  },

  uploadDocument: async (wsId, title, filename) => {
    const { data } = await api.post(`/workspaces/${wsId}/documents`, {
      title, filename, mime_type: 'application/octet-stream', content: '',
    });
    await get().fetchDocuments(wsId);
    return data;
  },

  approveDocument: async (docId) => {
    const { data } = await api.post(`/documents/${docId}/approve`);
    return data;
  },

  shareDocument: async (docId, targetUaId, operations) => {
    const { data } = await api.post(`/documents/${docId}/share`, {
      target_ua_id: targetUaId, operations,
    });
    return data;
  },

  publishDocument: async (docId) => {
    const { data } = await api.post(`/documents/${docId}/publish`);
    return data;
  },
}));

// ==================== MESSAGING STORE ====================
export const useMessagingStore = create((set, get) => ({
  channels: [],
  dms: [],
  activeChannel: null,
  messages: [],
  hasMore: false,
  loadingMessages: false,
  ws: null,
  typingUsers: {},

  fetchChannels: async (wsId) => {
    const { data } = await api.get(`/workspaces/${wsId}/channels`);
    set({ channels: data.channels || [] });
  },

  fetchDMs: async () => {
    const { data } = await api.get('/dms');
    set({ dms: data.channels || [] });
  },

  selectChannel: (channel) => {
    set({ activeChannel: channel, messages: [], hasMore: false });
    get().fetchMessages(channel.id);
    // Subscribe via WebSocket
    const ws = get().ws;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'subscribe', channel_id: channel.id }));
    }
  },

  createChannel: async (wsId, name, channelType) => {
    const { data } = await api.post(`/workspaces/${wsId}/channels`, {
      name, channel_type: channelType || 'workspace',
    });
    await get().fetchChannels(wsId);
    return data;
  },

  createDM: async (targetUserId, targetNgacNodeId) => {
    const { data } = await api.post('/dms', {
      target_user_id: targetUserId, target_ngac_node_id: targetNgacNodeId,
    });
    await get().fetchDMs();
    return data;
  },

  sendMessage: async (channelId, content) => {
    const { data } = await api.post(`/channels/${channelId}/messages`, { content });
    // Optimistic: message will arrive via WebSocket
    return data;
  },

  fetchMessages: async (channelId, before) => {
    set({ loadingMessages: true });
    const params = before ? `?before=${before}` : '';
    const { data } = await api.get(`/channels/${channelId}/messages${params}`);
    const msgs = data.messages || [];
    if (before) {
      set((s) => ({
        messages: [...msgs.reverse(), ...s.messages],
        hasMore: data.has_more || false,
        loadingMessages: false,
      }));
    } else {
      set({ messages: msgs.reverse(), hasMore: data.has_more || false, loadingMessages: false });
    }
  },

  addMessage: (msg) => {
    set((s) => {
      // Avoid duplicates
      if (s.messages.find((m) => m.id === msg.id)) return s;
      return { messages: [...s.messages, msg] };
    });
  },

  connectWebSocket: (token) => {
    const existing = get().ws;
    if (existing && existing.readyState === WebSocket.OPEN) return;

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws?token=${token}`;
    const ws = new WebSocket(wsUrl);
    let reconnectTimer = null;
    let reconnectDelay = 1000;

    ws.onopen = () => {
      reconnectDelay = 1000;
      // Re-subscribe to active channel
      const active = get().activeChannel;
      if (active) {
        ws.send(JSON.stringify({ type: 'subscribe', channel_id: active.id }));
      }
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'message' && data.message) {
          get().addMessage(data.message);
        } else if (data.type === 'typing') {
          set((s) => ({
            typingUsers: { ...s.typingUsers, [data.channel_id]: data.content },
          }));
          setTimeout(() => {
            set((s) => {
              const t = { ...s.typingUsers };
              delete t[data.channel_id];
              return { typingUsers: t };
            });
          }, 3000);
        }
      } catch {}
    };

    ws.onclose = () => {
      set({ ws: null });
      reconnectTimer = setTimeout(() => {
        reconnectDelay = Math.min(reconnectDelay * 2, 30000);
        get().connectWebSocket(token);
      }, reconnectDelay);
    };

    ws.onerror = () => ws.close();
    set({ ws });
  },

  disconnectWebSocket: () => {
    const ws = get().ws;
    if (ws) ws.close();
    set({ ws: null });
  },

  sendTyping: (channelId) => {
    const ws = get().ws;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'typing', channel_id: channelId }));
    }
  },

  fetchChannelMembers: async (channelId) => {
    const { data } = await api.get(`/channels/${channelId}/members`);
    return data.members || [];
  },

  addChannelMember: async (channelId, ngacNodeId) => {
    await api.post(`/channels/${channelId}/members`, { ngac_node_id: ngacNodeId });
  },
}));
