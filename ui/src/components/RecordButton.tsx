import { Show } from 'solid-js';
import type { Component } from 'solid-js';
import { store, startRecording, stopRecording } from '../api';

export const RecordButton: Component = () => {
  const status = () => store.recording.status;

  const handleClick = () => {
    if (status() === 'recording') {
      stopRecording();
    } else if (status() === '') {
      startRecording();
    }
  };

  const color = () => {
    if (status() === 'recording') return '#ef4444';
    if (status() === 'rendering') return '#f59e0b';
    return 'var(--text-secondary)';
  };

  return (
    <button
      onClick={handleClick}
      disabled={status() === 'rendering'}
      title={status() === 'recording' ? 'Stop recording' : status() === 'rendering' ? 'Rendering WAV...' : 'Start recording'}
      style={{
        background: 'none',
        border: 'none',
        cursor: status() === 'rendering' ? 'not-allowed' : 'pointer',
        display: 'flex',
        'align-items': 'center',
        gap: '0.4rem',
        color: color(),
        padding: '0.25rem 0.5rem',
        'border-radius': '4px',
        'font-size': '0.875rem',
        transition: 'color 0.2s',
      }}
    >
      <svg
        width="14"
        height="14"
        viewBox="0 0 24 24"
        style={{ animation: status() === 'recording' ? 'pulse 1s ease-in-out infinite' : 'none' }}
      >
        <circle cx="12" cy="12" r="10" fill={color()} />
      </svg>
      <Show when={status() === 'recording'}>REC</Show>
      <Show when={status() === 'rendering'}>Rendering…</Show>
      <Show when={status() === ''}>REC</Show>
    </button>
  );
};
