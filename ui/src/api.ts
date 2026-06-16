
import { createStore, reconcile } from 'solid-js/store';

// Types
export interface Device {
  id: number;
  name: string;
  is_input: boolean;
}


export interface EngineState {
  current_index: number;
  current_profile: string;
  total_profiles: number;
  setlist_name: string;
  soundfont?: string;
  midi_device?: string;
  gain: number;
}

export interface ActiveKey {
  velocity: number;
}

export type RecordStatus = '' | 'recording' | 'rendering';

export interface RecordingState {
  status: RecordStatus;
  startedAt?: string;
  wavPath?: string;
  midPath?: string;
}

export interface AppState {
  status: any | null;
  devices: Device[];
  engine: EngineState;
  setlist: SetlistProfile[];
  connected: boolean;
  activeKeys: {
    input: Record<number, ActiveKey>;
    output: Record<number, ActiveKey>;
  };
  recording: RecordingState;
}

// Global Store
export const [store, setStore] = createStore<AppState>({
  status: null,
  devices: [],
  engine: { current_index: 0, current_profile: '', total_profiles: 0, setlist_name: '', gain: 1.0 },
  setlist: [],
  connected: false,
  activeKeys: { input: {}, output: {} },
  recording: { status: '' },
});

// Helper for REST
const fetchJson = async <T>(url: string, options?: RequestInit): Promise<T> => {
  const res = await fetch(url, options);
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<T>;
};

// Actions
export const fetchStatus = async () => {
  try {
    const data = await fetchJson<any>('/api/v1/status');
    setStore('status', data);
    if (data.total_profiles !== undefined) {
      setStore('engine', reconcile(data));
    }
    // Always sync gain from status
    if (data.gain !== undefined) {
      setStore('engine', 'gain', data.gain);
    }
  } catch (e) {
    console.error('Fetch status error', e);
  }
};

export const setGain = (gain: number) =>
  fetch('/api/v1/gain', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ gain }),
  });


export const fetchDevices = async () => {
  try {
    const data = await fetchJson<Device[]>('/api/v1/midi/devices');
    setStore('devices', data);
  } catch (e) {
    console.error('Fetch devices error', e);
  }
};

export interface SetlistProfile {
  name: string;
  programs: {
    bank: number;
    program: number;
    channel: number;
  }[];
}

export const fetchSetlist = async () => {
  try {
    const data = await fetchJson<{ setlist: { profiles: SetlistProfile[] }; current_index: number }>('/api/v1/setlist');
    const profiles = data.setlist?.profiles || [];
    setStore('setlist', reconcile(profiles));
    setStore('engine', 'current_index', data.current_index);
    setStore('engine', 'total_profiles', profiles.length);
  } catch (e) {
    console.error('Fetch setlist error', e);
  }
};

// Setlist Controls
export const nextProfile = () => fetch('/api/v1/setlist/next', { method: 'POST' }).then(fetchSetlist);
export const prevProfile = () => fetch('/api/v1/setlist/prev', { method: 'POST' }).then(fetchSetlist);
export const gotoProfile = (index: number) => 
  fetch('/api/v1/setlist/goto', { 
    method: 'POST', 
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ index }) 
  }).then(fetchSetlist);
export const loadSetlist = (path: string) => 
  fetch('/api/v1/setlist', { 
    method: 'PUT', 
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }) 
  }).then(fetchSetlist).then(fetchStatus);

export const connectMidi = (device_id: number) =>
  fetch('/api/v1/midi/connect', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ device_id })
  }).then(fetchStatus).then(fetchDevices);

export const startRecording = () =>
  fetch('/api/v1/record/start', { method: 'POST' }).catch(console.error);

export const stopRecording = () =>
  fetch('/api/v1/record/stop', { method: 'POST' }).catch(console.error);


// WebSocket Connection
let ws: WebSocket | null = null;
let reconnectTimer: any = null;

export const connectWebSocket = () => {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/stream`;
  
  ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    setStore('connected', true);
    console.log('WS Connected');
    fetchStatus();
    fetchDevices();
    fetchSetlist();
    if (reconnectTimer) clearInterval(reconnectTimer);
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'profile_switch') {
          fetchSetlist();
      } else if (msg.type === 'midi_reconnect' || msg.type === 'midi_disconnect') {
          fetchDevices();
      } else if (msg.type === 'gain_changed') {
          setStore('engine', 'gain', (msg.data as any).gain);
      } else if (msg.type === 'record_start') {
        setStore('recording', { status: 'recording', startedAt: (msg.data as any).started_at });
      } else if (msg.type === 'record_rendering') {
        setStore('recording', 'status', 'rendering');
      } else if (msg.type === 'record_stop') {
        setStore('recording', {
          status: '',
          wavPath: (msg.data as any).wav,
          midPath: (msg.data as any).mid,
        });
      } else if (msg.type === 'StatusSync') {
         setStore('engine', msg.data);
      } else if (msg.type === 'note_on') {
        setStore('activeKeys', msg.source as 'input'|'output', msg.key, { velocity: msg.vel });
      } else if (msg.type === 'note_off') {
        setStore('activeKeys', msg.source as 'input'|'output', msg.key, undefined!);
      }
    } catch (e) {
      console.error('WS Parse Error', e);
    }
  };

  ws.onclose = () => {
    setStore('connected', false);
    console.log('WS Closed, reconnecting...');
    ws = null;
    reconnectTimer = setTimeout(connectWebSocket, 2000);
  };
  
  ws.onerror = () => {
    ws?.close();
  };
};

export const sendNoteAction = (action: 'note_on' | 'note_off', key: number, vel: number, target: 'input' | 'output') => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ action, key, vel, target }));
  }
};

