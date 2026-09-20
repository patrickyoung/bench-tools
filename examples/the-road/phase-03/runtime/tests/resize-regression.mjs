import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
// Exercise the production resize body without a GPU; emulate Three's integer framebuffer sizing.
const source=readFileSync(new URL('../src/renderer.js',import.meta.url),'utf8');
const body=source.split(' resize(){')[1].split('\n }\n')[0];
const resize=new Function('innerWidth','innerHeight',body);
for(const [width,height] of [[3840,1080],[1080,3840],[1440,900],[0,0]]){
 for(const currentDpr of [1,2,.6]){
  const w=Math.max(1,width),h=Math.max(1,height);
  const view={dpr:currentDpr,camera:{updateProjectionMatrix(){}},renderer:{
   setPixelRatio(d){this.dpr=d;},
   setSize(w,h,css){this.width=w;this.height=h;this.css=css;this.buffer=[Math.floor(w*this.dpr),Math.floor(h*this.dpr)];}
  },composer:{setPixelRatio(d){this.dpr=d;},setSize(w,h){this.width=w;this.height=h;}},
  fxaa:{uniforms:{resolution:{value:{set(x,y){this.x=x;this.y=y;}}}}},
  particleMaterial:{uniforms:{pixelRatio:{value:0}}}};
  resize.call(view,width,height);
  assert.equal(view.renderer.width,w);assert.equal(view.renderer.height,h);
  assert.equal(view.renderer.css,false);assert.equal(view.camera.aspect,w/h);
  assert.ok(Number.isFinite(view.dpr)&&view.dpr>0&&view.dpr<=currentDpr);
  assert.equal(view.dpr,Math.min(currentDpr,2560/w,1600/h,Math.sqrt(1800000/(w*h))));
  const [bw,bh]=view.renderer.buffer;
  assert.ok(bw<=2560&&bh<=1600&&bw*bh<=1800000);
  assert.equal(view.composer.dpr,view.dpr);assert.equal(view.composer.width,w);assert.equal(view.composer.height,h);
  assert.equal(view.particleMaterial.uniforms.pixelRatio.value,view.dpr);
  assert.equal(view.fxaa.uniforms.resolution.value.x,1/(w*view.dpr));
  assert.equal(view.fxaa.uniforms.resolution.value.y,1/(h*view.dpr));
  console.log(JSON.stringify({width,height,currentDpr,aspect:view.camera.aspect,dpr:view.dpr,buffer:[bw,bh]}));
 }
}
