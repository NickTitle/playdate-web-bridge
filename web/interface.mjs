import {PCMPlayer} from './audio.mjs';
import {Controls} from './controls.mjs';
const status = document.querySelector('#status');
const connect = document.querySelector('#connect');
const screen = document.querySelector('#screen');
const inputs = [...document.querySelectorAll('[data-button],#crank')];
let socket;
let audio;
let audioAvailable;
let audioReceived = false;
let lastAudioAt = 0;
const liveStatus = () => {
  const audioStatus = audioAvailable === false ? 'Audio disabled on server'
    : audioAvailable !== true ? 'Audio availability unverified'
    : audio && audio.context.state !== 'running' ? 'Audio paused; tap to resume'
    : audioReceived ? 'Experimental audio streaming'
    : audio ? 'Audio unlocked; waiting for stream'
    : 'Experimental audio requires Enable audio';
  status.textContent = `Live simulator connected. ${audioStatus}; iOS testing pending.`;
};
async function checkAudioAvailability() {
  try {
    const response = await fetch(new URL('./healthz', location.href), {cache: 'no-store'});
    if (!response.ok) throw new Error('Health check failed');
    const health = await response.json();
    if (typeof health.audio_available !== 'boolean') throw new Error('Unknown audio availability');
    const wasDisabled = audioAvailable === false;
    audioAvailable = health.audio_available;
    sound.disabled = audioAvailable === false;
    if (audioAvailable === false) sound.textContent = 'Audio disabled on server';
    else if (wasDisabled) sound.textContent = 'Enable audio';
    if (lastFrameAt && socket?.readyState === WebSocket.OPEN) liveStatus();
  } catch {
    audioAvailable = undefined;
    sound.disabled = false;
    if (sound.textContent === 'Audio disabled on server') sound.textContent = 'Enable audio';
    if (lastFrameAt && socket?.readyState === WebSocket.OPEN) liveStatus();
  }
}
const sound = document.querySelector('#sound');
const audioGain = document.querySelector('#audio-gain');
audioGain.oninput = () => {
 const db = Number(audioGain.value);
 document.querySelector('#audio-gain-label').textContent = `${db} dB`;
 if (audio) audio.gain = 10 ** (db / 20);
};
sound.onclick = async () => {
  try {
    if (audio?.context.state === 'running') {
      audio.reset(); audioReceived = false;
      sound.textContent = 'Audio paused · tap to resume';
      await audio.context.suspend();
      return;
    }
    if (!audio) {
      audio = new PCMPlayer(new (window.AudioContext || window.webkitAudioContext)(), Number(audioGain.value));
      audio.context.addEventListener('statechange', () => {
        if (audio.context.state !== 'running') {
          audio.reset(); audioReceived = false;
          if (audioAvailable !== false) sound.textContent = 'Audio paused · tap to resume';
        }
      });
    }
    await audio.context.resume();
    if (audioAvailable === false) return;
    sound.textContent = audioReceived ? 'Audio streaming · tap to pause' : 'Audio unlocked · waiting for stream';
  } catch { sound.textContent = 'Audio unavailable in this browser'; }
};
checkAudioAvailability();
let imageURL;
let frameConnection;
let lastFrameAt = 0;
const controls = new Controls(state => {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(state));
});
const enabled = value => inputs.forEach(el => { el.disabled = !value; });
enabled(false);
const frameFailed = () => {
  lastFrameAt = 0;
  controls.releaseAll(); enabled(false);
  if (socket?.readyState === WebSocket.OPEN)
    status.textContent = 'Simulator frame unavailable; controls released. Waiting for a valid live frame.';
};
screen.onload = () => {
  if (socket?.readyState !== WebSocket.OPEN || frameConnection !== socket) return;
  if (screen.naturalWidth !== 400 || screen.naturalHeight !== 240) return frameFailed();
  lastFrameAt = performance.now();
  enabled(true);
  liveStatus();
};
screen.onerror = frameFailed;
connect.onclick = () => {
  if (socket && socket.readyState < WebSocket.CLOSING) {
    release(); enabled(false); lastFrameAt = 0;
    connect.disabled = true;
    status.textContent = 'Disconnecting…';
    socket.close();
    return;
  }
  controls.releaseAll(); enabled(false); lastFrameAt = 0;
  frameConnection = undefined;
  audio?.reset();
  status.textContent = 'Connecting…';
  audioReceived = false;
  checkAudioAvailability();
  const url = new URL('./session', location.href);
  url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const connection = new WebSocket(url);
  socket = connection;
  socket.binaryType = 'arraybuffer';
  connection.onopen = () => {
    if (socket !== connection) return;
    controls.crank = 0;
    controls.releaseAll();
    status.textContent = 'Connected; waiting for live simulator frame. Audio requires Enable audio and server opt-in.';
    connect.disabled = false; connect.textContent = 'Disconnect';
  };
  connection.onmessage = event => {
    if (socket !== connection) return;
    if (!(event.data instanceof ArrayBuffer)) return;
    if (event.data.byteLength >= 4 && new DataView(event.data).getUint32(0) === 0x53465043) {
      if (!document.hidden && audio?.push(event.data)) {
        audioReceived = true;
        lastAudioAt = performance.now();
        sound.textContent = 'Audio streaming · tap to pause';
      }
      return;
    }
    frameConnection = connection;
    const previous = imageURL;
    imageURL = URL.createObjectURL(new Blob([event.data], {type: 'image/png'}));
    screen.src = imageURL;
    if (previous) URL.revokeObjectURL(previous);
  };
  connection.onclose = () => {
    if (socket !== connection) return;
    audio?.reset(); audioReceived = false;
    if (audioAvailable !== false) sound.textContent = !audio ? 'Enable audio'
      : audio.context.state === 'running' ? 'Audio unlocked · waiting for stream'
      : 'Audio paused · tap to resume';
    lastFrameAt = 0;
    controls.releaseAll(); enabled(false); connect.disabled = false; connect.textContent = 'Connect';
    status.textContent = 'Disconnected. Reconnect to resume; controls released.';
  };
  connection.onerror = () => { if (socket !== connection) return; status.textContent = 'Simulator connection failed. Deployment is not yet verified.'; };
};
// Explicitly consume control gestures as well as declaring touch-action:none.
// Chromium's trusted multi-touch path otherwise delayed timers/PCM delivery
// about 100 ms in the real-SDK probe. Keep normal page scrolling outside controls.
for (const type of ['touchstart', 'touchmove']) {
  document.querySelector('.controls').addEventListener(type, event => event.preventDefault(), {passive: false});
}
for (const el of inputs) {
  const angle = event => {
    const box = el.getBoundingClientRect();
    return Math.atan2(event.clientY-box.top-box.height/2, event.clientX-box.left-box.width/2)*180/Math.PI;
  };
  el.onpointerdown = event => {
    if (el.disabled || event.button !== 0) return;
    event.preventDefault();
    if (el.dataset.button === "menu") {
      controls.releaseAll();
      // Opening Menu is non-destructive. A browser modal blocks PCM scheduling
      // and frame delivery; keep the shared-save warning inline instead.
      // Checkpoint jumps retain their native deliberate confirmation.
    }
    el.setPointerCapture(event.pointerId);
    if (el.dataset.button) controls.press(event.pointerId, el.dataset.button);
    else controls.turnStart(event.pointerId, angle(event));
  };
  el.onpointermove = event => { if (!el.dataset.button) controls.turn(event.pointerId, angle(event)); };
  for (const name of ['pointerup', 'pointercancel', 'lostpointercapture']) {
    el.addEventListener(name, event => controls.release(event.pointerId));
  }
  el.oncontextmenu = event => event.preventDefault();
}
// Full snapshots repair a lost release; server must enforce a <=1 s lease.
setInterval(() => {
  if (audioReceived && performance.now() - lastAudioAt > 1000) {
    audio?.reset(); audioReceived = false;
    if (audioAvailable !== false) sound.textContent = 'Audio unlocked · waiting for stream';
  }
  // A connected socket (or continuing PCM) does not prove fresh game rendering.
  if (lastFrameAt && performance.now() - lastFrameAt > 1000) frameFailed();
  if (!document.hidden) controls.emit();
}, 250);
const release = () => {
  controls.releaseAll(); audio?.reset(); audioReceived = false;
  if (audioAvailable !== false && audio) sound.textContent = audio.context.state === 'running'
    ? 'Audio unlocked · waiting for stream' : 'Audio paused · tap to resume';
};
window.addEventListener('blur', release);
window.addEventListener('pagehide', () => { release(); socket?.close(); });
document.addEventListener('visibilitychange', () => { if (document.hidden) release(); });
