// Actual SDK PCM: 44.1 kHz stereo S16LE, with SFPC capture timestamp header.
export const PLAYBACK_GAIN_DB = 0;
export class PCMPlayer {
  constructor(context, gainDB = PLAYBACK_GAIN_DB, now = () => performance.now()) {
    if (!Number.isFinite(gainDB) || gainDB < 0 || gainDB > 30) throw new RangeError('playback gain must be 0..30 dB');
    this.now = now; this.transitFloor = undefined;
    this.gain = 10 ** (gainDB / 20);
    this.limitedSamples = 0;
    this.context = context; this.next = 0; this.sources = new Set(); this.dropped = 0; }
  push(packet) {
    const view = new DataView(packet);
    if (packet.byteLength <= 16 || packet.byteLength > 65552 || view.getUint32(0) !== 0x53465043 ||
        view.getUint32(4, true) !== packet.byteLength - 16 || (packet.byteLength - 16) % 4) return false;
    const ctx = this.context;
    if (ctx.state !== 'running') return false;
    // WebSocket/TCP can deliver old PCM after a network stall even though our
    // scheduled queue is short. Compare elapsed capture and arrival clocks;
    // their absolute epochs need not agree. Keep the fastest observed transit.
    const capturedMS = Number(view.getBigUint64(8, true)) / 1e6;
    const transit = this.now() - capturedMS;
    this.transitFloor = Math.min(this.transitFloor ?? transit, transit);
    if (transit - this.transitFloor > 150) {
      this.clearQueue(); this.dropped++; return false;
    }
    // Keep the newest 100 ms even if the backend sends a larger block. Bound
    // the END of playback, including this packet, rather than only its start.
    const packetFrames = (packet.byteLength - 16) / 4;
    const frames = Math.min(packetFrames, 4410);
    const offset = packetFrames - frames;
    if (offset) this.dropped++;
    const duration = frames / 44100;
    // Preserve contiguous queued audio while at least 5 ms remains to submit
    // the next source. Reapply startup lead only on a real near-underrun.
    let start = this.next >= ctx.currentTime + 0.005 ? this.next : ctx.currentTime + 0.02;
    if (start + duration > ctx.currentTime + 0.15) {
      this.clearQueue(); this.dropped++;
      start = ctx.currentTime + 0.02;
    }
    const buffer = ctx.createBuffer(2, frames, 44100);
    for (let channel = 0; channel < 2; channel++) {
      const samples = buffer.getChannelData(channel);
      for (let i = 0; i < frames; i++) {
        const value = view.getInt16(16 + (i + offset) * 4 + channel * 2, true) / 32768 * this.gain;
        // Emergency ceiling, not automatic normalization. Normal audited SDK
        // levels stay below this; unexpected loud material can distort here.
        if (Math.abs(value) > 0.95) this.limitedSamples++;
        samples[i] = Math.max(-0.95, Math.min(0.95, value));
      }
    }
    const source = ctx.createBufferSource();
    source.buffer = buffer; source.connect(ctx.destination);
    this.sources.add(source);
    source.onended = () => { this.sources.delete(source); source.disconnect(); };
    source.start(start); this.next = start + buffer.duration;
    return true;
  }
  reset() { this.transitFloor = undefined; this.clearQueue(); }
  clearQueue() { for (const source of this.sources) { source.stop(); source.disconnect(); } this.sources.clear(); this.next = 0; }
}
