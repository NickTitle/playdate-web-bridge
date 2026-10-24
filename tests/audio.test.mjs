import {test} from 'node:test';
import assert from 'node:assert/strict';
import {PCMPlayer} from '../web/audio.mjs';
function fixture(gainDB = 0) {
  const ctx = {state:'running', currentTime:1, destination:{}, starts:[], stops:0,
    createBuffer(channels, frames, rate) {
      const data = [new Float32Array(frames), new Float32Array(frames)];
      this.data = data; return {duration:frames/rate, getChannelData:c=>data[c]};
    },
    createBufferSource() { return {connect(){}, disconnect(){}, start:t=>ctx.starts.push(t), stop:()=>ctx.stops++}; }
  };
  const packet = new ArrayBuffer(16+4096); const v = new DataView(packet);
  v.setUint32(0,0x53465043); v.setUint32(4,4096,true); v.setBigUint64(8,1n,true);
  v.setInt16(16,-32768,true); v.setInt16(18,16384,true);
  return {ctx,packet,player:new PCMPlayer(ctx, gainDB)};
}
test('unity PCM preserves stereo levels below the safety ceiling and sample duration',()=>{
  const {ctx,packet,player}=fixture(); assert.equal(player.push(packet),true);
  assert.ok(Math.abs(ctx.data[0][0]+0.95)<1e-7); assert.equal(ctx.data[1][0],0.5);
  assert.equal(ctx.starts[0],1.02); assert.equal(player.next,1.02+1024/44100);
});
test('stalled playback has bounded scheduled audio and reset cancels sources',()=>{
  const {ctx,packet,player}=fixture();
  for(let i=0;i<1000;i++) player.push(packet);
  assert.ok(player.next<=ctx.currentTime+0.15); assert.ok(player.sources.size<=7);
  assert.ok(player.dropped>0); assert.ok(ctx.stops>0);
  player.reset(); assert.equal(player.sources.size,0); assert.equal(player.next,0);
});
test('suspended context and malformed packets are rejected',()=>{
  const {ctx,packet,player}=fixture(); ctx.state='suspended';
  assert.equal(player.push(packet),false); ctx.state='running';
  assert.equal(player.push(new ArrayBuffer(2)),false);
  new DataView(packet).setUint32(4,4,true); assert.equal(player.push(packet),false);
  assert.equal(ctx.starts.length,0);
});

test('largest accepted packet retains newest samples and cannot exceed latency bound',()=>{
  const {ctx,player}=fixture();
  const packet=new ArrayBuffer(16+65536); const view=new DataView(packet);
  view.setUint32(0,0x53465043); view.setUint32(4,65536,true);
  const offset=16384-4410;
  view.setInt16(16+offset*4,8192,true);
  view.setInt16(packet.byteLength-2,-16384,true);
  for(let i=0;i<20;i++) {
    assert.equal(player.push(packet),true);
    assert.ok(player.next<=ctx.currentTime+0.15);
    assert.equal(player.sources.size,1);
  }
  assert.equal(ctx.data[0].length,4410);
  assert.equal(ctx.data[0][0],0.25);
  assert.equal(ctx.data[1][4409],-0.5);
  assert.ok(ctx.stops>=19);
});
test('queue resets before an incoming block would cross the playback-end bound',()=>{
  const {ctx,packet,player}=fixture();
  player.next=ctx.currentTime+0.14;
  player.push(packet);
  assert.equal(ctx.starts[0],ctx.currentTime+0.02);
  assert.equal(player.dropped,1);
  assert.ok(player.next<=ctx.currentTime+0.15);
});

test('experimental gain lifts quiet stereo PCM and caps unexpected overload',()=>{
  const {ctx,packet,player}=fixture(30);
  const view=new DataView(packet);
  view.setInt16(16,100,true); view.setInt16(18,-200,true);
  player.push(packet);
  assert.ok(Math.abs(ctx.data[0][0]-100/32768*10**1.5)<1e-7);
  assert.ok(Math.abs(ctx.data[1][0]+200/32768*10**1.5)<1e-7);
  assert.equal(player.limitedSamples,0);
  view.setInt16(16,32767,true); view.setInt16(18,-32768,true);
  player.push(packet);
  assert.ok(Math.abs(ctx.data[0][0]-0.95)<1e-7);
  assert.ok(Math.abs(ctx.data[1][0]+0.95)<1e-7);
  assert.equal(player.limitedSamples,2);
});
test('invalid playback gain is rejected',()=>{
  const {ctx}=fixture();
  for(const gain of [NaN,Infinity,-1,31]) assert.throws(()=>new PCMPlayer(ctx,gain),RangeError);
});

test('arrival jitter preserves contiguous playback when queued lead is still safe',()=>{
  const {ctx,packet,player}=fixture();
  player.push(packet);
  const end=player.next;
  ctx.currentTime=end-0.010;
  player.push(packet);
  assert.equal(ctx.starts[1],end);
  assert.equal(player.dropped,0);
});
test('near-underrun restores startup lead without scheduling into the past',()=>{
  const {ctx,packet,player}=fixture();
  player.push(packet);
  ctx.currentTime=player.next-0.002;
  player.push(packet);
  assert.equal(ctx.starts[1],ctx.currentTime+0.02);
  assert.ok(player.next<=ctx.currentTime+0.15);
});

// A short Web Audio queue alone does not bound TCP backlog latency.
test('network stall discards stale PCM until capture clock catches up',()=>{
  const {ctx,packet}=fixture(); let now=1000;
  const player=new PCMPlayer(ctx,0,()=>now);
  const stamp=ms=>new DataView(packet).setBigUint64(8,BigInt(ms)*1000000n,true);
  stamp(5000); assert.equal(player.push(packet),true);
  now=1500; stamp(5023); assert.equal(player.push(packet),false);
  assert.equal(player.sources.size,0);
  now=1523; stamp(5046); assert.equal(player.push(packet),false);
  stamp(5523); assert.equal(player.push(packet),true);
  assert.equal(player.dropped,2);
  // Explicit reconnect/resume resets clock calibration, not a queue overflow.
  player.reset(); stamp(100); assert.equal(player.push(packet),true);
});
