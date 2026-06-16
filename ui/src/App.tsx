import { onMount, For, Show, createSignal } from 'solid-js';
import type { Component } from 'solid-js';
import { 
  store, 
  connectWebSocket, 
  nextProfile, 
  prevProfile, 
  gotoProfile,
  connectMidi,
  loadSetlist,
  setGain
} from './api';
import { t, toggleLocale, locale } from './i18n';
import { PianoKeyboard } from './components/PianoKeyboard';
import { RecordButton } from './components/RecordButton';

const App: Component = () => {
  const [loadPath, setLoadPath] = createSignal('');

  onMount(() => {
    connectWebSocket();
  });

  return (
    <>
      <header class="header">
        <div style="display: flex; align-items: center; gap: 1rem;">
          <h1 class="header-title">{t('header.title')}</h1>
          <button class="btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" onClick={toggleLocale}>
            {locale() === 'en' ? '中' : 'En'}
          </button>
          <RecordButton />
        </div>
        <div class={`status-badge ${store.connected ? 'status-online' : 'status-offline'}`}>
          <div class="status-dot"></div>
          {store.connected ? t('header.connected') : t('header.disconnected')}
        </div>
      </header>

      <main class="container animate-fade-in">
        
        <div class="dashboard-grid">
          {/* Controls Card */}
          <div class="card animate-slide-up" style="animation-delay: 0.1s;">
            <h2 class="card-title">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20V10"></path><path d="M18 20V4"></path><path d="M6 20v-4"></path></svg>
              {t('controls.title')}
            </h2>
            
            <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
              <button class="btn btn-secondary" style="flex: 1" onClick={prevProfile} disabled={!store.engine || store.engine.current_index <= 0}>
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"></polyline></svg>
                {t('controls.prev')}
              </button>
              <button class="btn btn-primary" style="flex: 1" onClick={nextProfile} disabled={!store.engine || store.engine.current_index >= store.engine.total_profiles - 1}>
                {t('controls.next')}
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"></polyline></svg>
              </button>
            </div>

            {/* Volume Slider */}
            <div style="margin-top: 1.25rem; padding-top: 1.25rem; border-top: 1px solid rgba(255,255,255,0.07);">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                <p style="color: var(--text-secondary); font-size: 0.875rem; display: flex; align-items: center; gap: 0.4rem;">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path></svg>
                  {t('controls.volume')}
                </p>
                <span style="font-size: 0.875rem; font-weight: 600; color: var(--accent-color); min-width: 3rem; text-align: right;">
                  {Math.round(store.engine.gain * 100)}%
                </span>
              </div>
              <input
                type="range"
                min="0" max="3" step="0.01"
                value={store.engine.gain}
                style="width: 100%; accent-color: var(--accent-color); cursor: pointer;"
                onInput={(e) => setGain(parseFloat(e.currentTarget.value))}
              />
              <div style="display: flex; justify-content: space-between; font-size: 0.7rem; color: var(--text-secondary); margin-top: 0.25rem;">
                <span>0%</span>
                <span>100%</span>
                <span>200%</span>
                <span>300%</span>
              </div>
            </div>

            <Show when={store.engine?.soundfont}>
              <div style="margin-top: 1.5rem; padding-top: 1.5rem; border-top: 1px solid rgba(255,255,255,0.1);">
                <p style="color: var(--text-secondary); font-size: 0.875rem;">{t('controls.active_soundfont')}</p>
                <div style="font-family: monospace; word-break: break-all; margin-top: 0.25rem;">
                  {store.engine?.soundfont}
                </div>
              </div>
            </Show>
          </div>

          {/* MIDI Devices Card */}
          <div class="card animate-slide-up" style="animation-delay: 0.2s;">
            <h2 class="card-title">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"></path><circle cx="6" cy="18" r="3"></circle><circle cx="18" cy="16" r="3"></circle></svg>
              {t('midi.title')}
            </h2>
            
            <Show when={store.engine?.midi_device}>
              <div style="margin-bottom: 1rem; padding: 0.75rem; background: rgba(16, 185, 129, 0.1); border-radius: var(--radius-sm); border: 1px solid rgba(16, 185, 129, 0.2);">
                <span class="status-badge status-online" style="padding: 0; background: none; margin-bottom: 0.25rem;">
                  <div class="status-dot"></div> {t('midi.active')}
                </span>
                <div style="font-family: monospace;">{store.engine?.midi_device}</div>
              </div>
            </Show>

            <div style="display: flex; flex-direction: column; gap: 0.5rem; max-height: 200px; overflow-y: auto;">
              <For each={store.devices.filter(d => d.is_input)} fallback={<div style="color: var(--text-secondary)">{t('midi.no_devices')}</div>}>
                {(dev) => (
                  <div style="display: flex; justify-content: space-between; align-items: center; padding: 0.5rem; background: var(--bg-card); border-radius: var(--radius-sm);">
                    <div style="display: flex; flex-direction: column; max-width: 70%;">
                      <span style="font-size: 0.875rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 500;" title={dev.name}>{dev.name}</span>
                      <span style="font-size: 0.75rem; color: var(--text-secondary);">{t('midi.device_id')}: {dev.id}</span>
                    </div>
                    <button class="btn btn-secondary" style="padding: 0.25rem 0.75rem; font-size: 0.75rem;" onClick={() => connectMidi(dev.id)}>{t('midi.connect')}</button>
                  </div>
                )}
              </For>
            </div>
          </div>

        </div>

        {/* Setlist Card */}
        <div class="dashboard-grid" style="grid-template-columns: 1fr;">
          <div class="card animate-slide-up" style="animation-delay: 0.3s;">
            <h2 class="card-title" style="justify-content: space-between;">
              <div style="display: flex; align-items: center; gap: 0.5rem;">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line></svg>
                {t('setlist.title')}
              </div>
              
              <div style="display: flex; gap: 0.5rem;">
                <input 
                  type="text" 
                  value={loadPath()} 
                  onInput={(e) => setLoadPath(e.currentTarget.value)}
                  placeholder={t('setlist.placeholder')}
                  style="background: var(--bg-card); border: 1px solid rgba(255,255,255,0.1); border-radius: var(--radius-sm); color: white; padding: 0.25rem 0.5rem;"
                />
                <button class="btn btn-secondary" style="padding: 0.25rem 0.75rem; font-size: 0.875rem;" onClick={() => loadSetlist(loadPath())}>{t('setlist.load')}</button>
              </div>
            </h2>
            
            <div style="margin-top: 1rem; max-height: 400px; overflow-y: auto; padding-right: 0.5rem;">
              <For each={store.setlist} fallback={<div style="color: var(--text-secondary); padding: 1rem; text-align: center;">{t('setlist.no_load')}</div>}>
                {(item, index) => (
                  <div 
                    class={`setlist-item ${store.engine.current_index === index() ? 'active' : ''}`}
                    onClick={() => gotoProfile(index())}
                    style="cursor: pointer;"
                  >
                    <div style="display: flex; align-items: center; max-width: 70%;">
                      <span class="item-index">#{index() + 1}</span>
                      <span class="item-title">{item.name}</span>
                    </div>
                    <div style="color: var(--text-secondary); font-family: monospace; font-size: 0.875rem;">
                      B:{item.programs?.[0]?.bank ?? '-'} P:{item.programs?.[0]?.program ?? '-'}
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>
        </div>

      </main>
      <PianoKeyboard />
    </>
  );
};

export default App;
