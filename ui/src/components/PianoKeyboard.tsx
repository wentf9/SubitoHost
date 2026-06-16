import { For } from 'solid-js';
import type { Component } from 'solid-js';
import { store, sendNoteAction } from '../api';
import './PianoKeyboard.css';

// 88 keys: A0 (21) to C8 (108)
const START_KEY = 21;
const END_KEY = 108;

const isBlackKey = (midiNote: number) => {
  const note = midiNote % 12;
  return [1, 3, 6, 8, 10].includes(note);
};

const keys = Array.from({ length: END_KEY - START_KEY + 1 }, (_, i) => ({
  note: START_KEY + i,
  isBlack: isBlackKey(START_KEY + i)
}));

const PianoRow: Component<{ type: 'input' | 'output', label: string }> = (props) => {
  const handlePointerDown = (e: PointerEvent, note: number) => {
    (e.target as HTMLElement).releasePointerCapture(e.pointerId); // allow sliding
    sendNoteAction('note_on', note, 100, props.type);
  };
  
  const handlePointerUp = (_e: PointerEvent, note: number) => {
    sendNoteAction('note_off', note, 0, props.type);
  };

  const handlePointerEnter = (e: PointerEvent, note: number) => {
    if (e.buttons > 0) {
      sendNoteAction('note_on', note, 100, props.type);
    }
  };

  const handlePointerLeave = (e: PointerEvent, note: number) => {
    if (e.buttons > 0) {
      sendNoteAction('note_off', note, 0, props.type);
    }
  };

  return (
    <div class="piano-row" onContextMenu={(e) => e.preventDefault()}>
      <div class="piano-row-label">{props.label}</div>
      <For each={keys}>
        {(key) => {
          // Reactive velocity check
          const activeKey = () => store.activeKeys[props.type][key.note];
          const velocity = () => activeKey()?.velocity || 0;
          
          // Calculate gradient based on velocity (0-127)
          const highlightStyle = () => {
            if (velocity() === 0) return { display: 'none' };
            const intensity = velocity() / 127;
            const color = props.type === 'input' ? '16, 185, 129' : '59, 130, 246'; // Green for input, Blue for output
            return {
              background: `linear-gradient(to top, rgba(${color}, ${0.5 + 0.5*intensity}) 0%, transparent ${30 + 70*intensity}%)`,
              'box-shadow': `0 0 ${10 * intensity}px rgba(${color}, ${0.5 * intensity})`
            };
          };

          return (
            <div
              class={`key ${key.isBlack ? 'black' : 'white'}`}
              onPointerDown={(e) => handlePointerDown(e, key.note)}
              onPointerUp={(e) => handlePointerUp(e, key.note)}
              onPointerEnter={(e) => handlePointerEnter(e, key.note)}
              onPointerLeave={(e) => handlePointerLeave(e, key.note)}
            >
              <div class="highlight" style={highlightStyle()}></div>
            </div>
          );
        }}
      </For>
    </div>
  );
};

export const PianoKeyboard: Component = () => {
  return (
    <div class="piano-container">
      <PianoRow type="input" label="RAW INPUT" />
      <PianoRow type="output" label="PROCESSED OUTPUT" />
    </div>
  );
};
