// Transport-independent touch state. The server must expire held keys on disconnect.
export class Controls {
  constructor(send) {
    this.send = send;
    this.pointers = new Map();
    this.crank = 0;
    this.sequence = 0;
  }
  snapshot() {
    return {sequence: ++this.sequence,
      buttons: [...new Set([...this.pointers.values()].filter(v => typeof v === 'string'))].sort(),
      crank: this.crank};
  }
  emit() { this.send(this.snapshot()); }
  press(id, button) {
    if (!['up', 'down', 'left', 'right', 'a', 'b', 'menu'].includes(button)) throw Error('Unknown button');
    this.pointers.set(id, button);
    this.emit();
  }
  turnStart(id, angle) {
    // One physical crank has one gesture owner. A second touching finger must
    // not add its angle changes to the first finger's rotation.
    if ([...this.pointers.values()].some(value => typeof value !== 'string')) return;
    this.pointers.set(id, {angle});
  }
  turn(id, angle) {
    const state = this.pointers.get(id);
    if (!state || typeof state === 'string') return;
    // Unwrap across the -180/+180 boundary; clockwise is positive.
    const delta = ((angle - state.angle + 540) % 360) - 180;
    state.angle = angle;
    this.crank += delta;
    this.emit();
  }
  release(id) { if (this.pointers.delete(id)) this.emit(); }
  releaseAll() { this.pointers.clear(); this.emit(); }
}
