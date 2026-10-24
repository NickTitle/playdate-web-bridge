import {createRequire} from 'node:module';
import {readFile,mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
import {join} from 'node:path';
const {chromium}=createRequire(import.meta.url)(process.env.PLAYDATE_BRIDGE_PLAYWRIGHT || 'playwright-core');
const root=process.env.PLAYDATE_BRIDGE_ROOT || fileURLToPath(new URL('..',import.meta.url));
const html=(await readFile(join(root,'web/index.html'),'utf8')).replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi,'');
const browser=await chromium.launch({executablePath:process.env.PLAYDATE_BRIDGE_CHROMIUM,headless:true,args:['--no-sandbox']});
const output=join(root,'build/layout');
await mkdir(output,{recursive:true});
const views=[[320,568],[568,320],[667,280],[375,667],[390,664],[430,740],[667,375],[844,390],[1000,1060]];
try {
 for(const [width,height] of views){
  const page=await browser.newPage({viewport:{width,height}});
  await page.setContent(html);
  await page.locator('#status').evaluate(el=>el.textContent='Live simulator connected. Experimental audio requires Enable audio; iOS testing pending.');
  const boxes=await page.locator('#screen,[data-button],#crank').evaluateAll(els=>els.map(el=>{const r=el.getBoundingClientRect();return {id:el.id||el.dataset.button,x:r.x,y:r.y,width:r.width,height:r.height,cx:r.x+r.width/2,cy:r.y+r.height/2};}));
  for(const b of boxes){
   assert.ok(b.x>=-0.5 && b.y>=-0.5 && b.x+b.width<=width+1 && b.y+b.height<=height+1,`${width}x${height}: ${b.id} outside viewport ${JSON.stringify(b)}`);
   if(b.id!=='screen') assert.ok(b.width>=43.9 && b.height>=43.9,`${width}x${height}: ${b.id} touch target <44px`);
  }
  for(let i=0;i<boxes.length;i++)for(let j=i+1;j<boxes.length;j++){
   const a=boxes[i],b=boxes[j];
   assert.ok(a.x+a.width<=b.x+0.5 || b.x+b.width<=a.x+0.5 || a.y+a.height<=b.y+0.5 || b.y+b.height<=a.y+0.5,`${width}x${height}: ${a.id}/${b.id} overlap`);
  }
  const byId=Object.fromEntries(boxes.map(b=>[b.id,b]));
  const dpad=await page.locator('.dpad').evaluate(el=>{const r=el.getBoundingClientRect();return {cx:r.x+r.width/2,cy:r.y+r.height/2};});
  assert.ok(byId.b.cx<byId.a.cx,`${width}x${height}: B should be left of A`);
  assert.ok(Math.abs(byId.a.cy-byId.b.cy)<=1,`${width}x${height}: A/B should be level`);
  assert.ok(dpad.cx<byId.b.cx,`${width}x${height}: Dpad should be left of B/A`);
  assert.ok(byId.b.cy<=dpad.cy+1,`${width}x${height}: A/B should sit at or above Dpad center`);
  assert.ok(byId.menu.cy<byId.screen.cy,`${width}x${height}: Menu should be upper beside display`);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`${width}x${height}: horizontal overflow`);
  await page.screenshot({path:join(output,`${width}x${height}.png`)});
  console.log(`${width}x${height}: hardware order, visibility, spacing and 44px targets passed`);
  await page.close();
 }
}finally{await browser.close();}
