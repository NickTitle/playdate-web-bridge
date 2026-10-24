import {test} from 'node:test';
import assert from 'node:assert/strict';
import {Controls} from '../web/controls.mjs';
test('held thrust and steering survive independent pointer releases', () => {
  const sent=[]; const c=new Controls(s=>sent.push(s));
  c.press(1,'up'); c.press(2,'left'); c.press(3,'a'); c.release(3);
  assert.deepEqual(sent.at(-1).buttons,['left','up']);
  c.release(2); assert.deepEqual(sent.at(-1).buttons,['up']);
  c.releaseAll(); assert.deepEqual(sent.at(-1).buttons,[]);
});
test('two fingers on B keep one held state until both release', () => {
  const sent=[]; const c=new Controls(s=>sent.push(s));
  c.press(1,'b'); c.press(2,'b'); c.release(1); c.release(2);
  assert.deepEqual(sent.map(s=>s.buttons),[['b'],['b'],['b'],[]]);
  assert.deepEqual(sent.map(s=>s.sequence),[1,2,3,4]);
});
test('crank unwraps both directions while thrust stays held', () => {
  const sent=[]; const c=new Controls(s=>sent.push(s));
  c.press(1,'up'); c.turnStart(2,170); c.turn(2,-170);
  assert.equal(c.crank,20); c.turn(2,170); assert.equal(c.crank,0);
  c.turn(2,80); assert.equal(c.crank,-90);
  assert.deepEqual(sent.at(-1).buttons,['up']);
  c.releaseAll(); c.turn(2,100); assert.equal(c.crank,-90);
});
test('crank has one finger owner while other buttons remain independent', () => {
  const sent=[]; const c=new Controls(s=>sent.push(s));
  c.press(1,'up'); c.turnStart(2,0); c.turnStart(3,90);
  c.turn(2,30); c.turn(3,120);
  assert.equal(c.crank,30, 'a second finger must not add another rotation');
  c.release(3); c.turn(2,60); assert.equal(c.crank,60);
  assert.deepEqual(sent.at(-1).buttons,['up']);
  c.release(2); c.turnStart(3,120); c.turn(3,150);
  assert.equal(c.crank,90, 'a new gesture can acquire the released crank');
  c.releaseAll(); c.turn(3,180); assert.equal(c.crank,90);
});
