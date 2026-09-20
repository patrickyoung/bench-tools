import {makePanorama} from './panorama.js';
import * as T from 'three';
import {EffectComposer} from 'three/addons/postprocessing/EffectComposer.js';
import {RenderPass} from 'three/addons/postprocessing/RenderPass.js';
import {OutputPass} from 'three/addons/postprocessing/OutputPass.js';
import {ShaderPass} from 'three/addons/postprocessing/ShaderPass.js';
import {FXAAShader} from 'three/addons/shaders/FXAAShader.js';
import {HDRLoader} from 'three/addons/loaders/HDRLoader.js';
import {B,L,R,SIZE} from './config.js';
import {integer,localRender} from './coordinates.js';
import {terrain,support,offset,authored,at,center,profile} from './generation.js';
import {Atmosphere,localParticle} from './atmosphere.js';
import {Assets,makeChunk,disposeChunk,makeLandmarks} from './meshing.js';
export class View{
 constructor(canvas){
  this.canvas=canvas;this.scene=new T.Scene();this.scene.fog=new T.Fog(L.fog.color,L.fog.near,L.fog.far);
  this.camera=new T.PerspectiveCamera(66,1,.07,12000);this.camera.rotation.order='YXZ';
  this.renderer=new T.WebGLRenderer({canvas,antialias:false,alpha:false,powerPreference:'high-performance'});
  this.renderer.outputColorSpace=T.SRGBColorSpace;this.renderer.toneMapping=T.ACESFilmicToneMapping;this.renderer.toneMappingExposure=L.exposure;
  this.renderer.shadowMap.enabled=true;this.renderer.shadowMap.type=T.PCFSoftShadowMap;this.renderer.info.autoReset=false;
  this.assets=new Assets();this.landmarks=makeLandmarks(this.assets);this.scene.add(this.landmarks.root);
  this.panorama=makePanorama();this.scene.add(this.panorama.root);
  this.hemisphere=new T.HemisphereLight(L.lights[0].color,B.palette.shadow,L.lights[0].intensity);this.scene.add(this.hemisphere);
  this.sun=new T.DirectionalLight(L.lights[1].color,L.lights[1].intensity);this.sun.castShadow=true;this.sun.shadow.mapSize.set(2048,2048);
  Object.assign(this.sun.shadow.camera,{left:-58,right:58,top:58,bottom:-58,near:1,far:240});
  this.sun.shadow.normalBias=.04;this.sun.shadow.bias=-.00015;this.scene.add(this.sun,this.sun.target);
  this.composer=new EffectComposer(this.renderer);this.renderPass=new RenderPass(this.scene,this.camera);this.outputPass=new OutputPass();this.composer.addPass(this.renderPass);this.composer.addPass(this.outputPass);this.fxaa=new ShaderPass(FXAAShader);this.composer.addPass(this.fxaa);
  this.skyMaterial=new T.ShaderMaterial({side:T.BackSide,depthWrite:false,uniforms:{},vertexShader:`
   varying vec3 direction;
   void main(){direction=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}`,
   fragmentShader:`precision highp float; varying vec3 direction;
   float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
   float noise2(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1.,0.)),f.x),mix(hash(i+vec2(0.,1.)),hash(i+1.),f.x),f.y);}
   void main(){
    vec3 d=normalize(direction);float h=max(d.y,0.);
    vec3 horizon=vec3(.64,.68,.62),zenith=vec3(.085,.23,.43);
    vec3 c=mix(horizon,zenith,pow(h,.34));
    vec3 sun=normalize(vec3(.68,.24,-.70));
    float glow=pow(max(dot(d,sun),0.),13.);c+=vec3(.22,.14,.05)*glow;
    float disc=smoothstep(.99986,.99997,dot(d,sun));c=mix(c,vec3(3.1,2.45,1.5),disc);
    vec2 cloudUV=d.xz/max(.13,d.y)*vec2(2.2,3.4);
    float clouds=noise2(cloudUV)*.58+noise2(cloudUV*2.8)*.27+noise2(cloudUV*7.)*.15;
    float veil=smoothstep(.57,.78,clouds)*smoothstep(.06,.22,d.y)*(1.-smoothstep(.55,.86,d.y));
    c=mix(c,vec3(.80,.79,.73),veil*.27);gl_FragColor=vec4(c,1.);
   }`});
  this.skyGeometry=new T.SphereGeometry(10000,32,16);this.sky=new T.Mesh(this.skyGeometry,this.skyMaterial);this.sky.frustumCulled=false;this.sky.renderOrder=-1000;this.scene.add(this.sky);
  this.time={value:0};this.waterTime={value:0};this.motion={value:1};
  // WebGL-only compatible shader extension of a physical dielectric surface.
  const wm=this.landmarks.water.material;wm.onBeforeCompile=shader=>{
   shader.uniforms.uRoadTime=this.waterTime;
   shader.vertexShader=shader.vertexShader.replace('#include <common>','#include <common>\nuniform float uRoadTime; varying vec3 vWaterPosition; varying vec2 vWaterUV;').replace('#include <begin_vertex>','#include <begin_vertex>\nvWaterPosition=position; vWaterUV=uv; transformed.y += .008*sin(position.x*2.7+uRoadTime*1.7)*sin(position.z*6.);');
   shader.fragmentShader=shader.fragmentShader.replace('#include <common>','#include <common>\nuniform float uRoadTime; varying vec3 vWaterPosition; varying vec2 vWaterUV;')
    .replace('#include <normal_fragment_begin>','#include <normal_fragment_begin>\nnormal=normalize(normal+vec3(.055*sin(vWaterPosition.x*3.1-uRoadTime*1.5)+.025*cos(vWaterPosition.z*5.+uRoadTime),.025*cos(vWaterPosition.z*7.+uRoadTime),0.));')
    .replace('#include <alphatest_fragment>','#include <alphatest_fragment>\nfloat shore=smoothstep(0.,.12,vWaterUV.y)*smoothstep(0.,.12,1.-vWaterUV.y);diffuseColor.a*=shore;');
  };wm.customProgramCacheKey=()=>'road-water-hdr-v2';
  this.makeParticles();this.groups=new Set();this.dpr=Math.min(devicePixelRatio||1,B.budgets.dpr);this.slow=0;this.fast=0;this.frames=0;this.disposed=false;this.resize();
  this.ready=Promise.all([this.assets.ready,this.loadEnvironment()]).then(()=>{this.assetsReady=!this.disposed;});
 }
 async loadEnvironment(){
  const hdr=await new HDRLoader().loadAsync(new URL('./assets/dry_field_1k.hdr',import.meta.url).href);
  if(this.disposed){hdr.dispose();return;}
  const pmrem=new T.PMREMGenerator(this.renderer);
  try{
   this.environmentTarget=pmrem.fromEquirectangular(hdr);
   this.scene.environment=this.environmentTarget.texture;this.scene.environmentIntensity=L.environment.intensity;
   this.scene.environmentRotation.set(0,L.environment.rotationYRadians,0);
  }finally{hdr.dispose();pmrem.dispose();}
 }
 makeParticles(){
  this.atmosphere=new Atmosphere();this.lastParticleSamples=[];this.lastParticleTime=0;
  this.particleGeometry=new T.BufferGeometry();this.particleCount=0;
  this.positions=new Float32Array(350*3);this.kinds=new Float32Array(350);this.sizes=new Float32Array(350);this.opacities=new Float32Array(350);
  for(const [name,array,size] of [['position',this.positions,3],['kind',this.kinds,1],['pointSize',this.sizes,1],['opacity',this.opacities,1]])
   this.particleGeometry.setAttribute(name,new T.BufferAttribute(array,size));
  this.particleMaterial=new T.ShaderMaterial({transparent:true,depthWrite:false,uniforms:{pixelRatio:{value:this.dpr||1}},vertexShader:`
   attribute float kind; attribute float pointSize; attribute float opacity; uniform float pixelRatio;
   varying float vKind; varying float vAlpha;
   void main(){
    vKind=kind;vAlpha=opacity;vec4 mv=modelViewMatrix*vec4(position,1.);
    gl_PointSize=clamp(pointSize*pixelRatio*700./max(1.,-mv.z),.5,44.);
    gl_Position=projectionMatrix*mv;
   }`,fragmentShader:`
   varying float vKind; varying float vAlpha;
   void main(){float d=length(gl_PointCoord-.5)*2.;if(d>1.)discard;
    vec3 c=vKind>1.5?vec3(.37,.40,.43):(vKind>.5?vec3(.28,.26,.13):vec3(.38,.30,.18));
    gl_FragColor=vec4(c,pow(1.-d,2.)*vAlpha);
   }`});
  this.particles=new T.Points(this.particleGeometry,this.particleMaterial);this.particles.frustumCulled=false;this.scene.add(this.particles);
 }
 updateParticles(player,time,reduced){
  const sample=this.atmosphere.sample(player,time,reduced);
  this.particleCount=sample.length;this.lastParticleTime=time;this.lastParticleReduced=reduced;
  sample.forEach((p,i)=>{
   this.positions.set(localParticle(p.worldPosition,player.chunk),i*3);
   this.kinds[i]=p.kind==='dust'?0:p.kind==='midge'?1:2;this.sizes[i]=p.size;this.opacities[i]=p.opacity;
   p.renderLocal=Array.from(this.positions.subarray(i*3,i*3+3));
  });
  this.lastParticleOrigin=[...player.chunk];this.lastParticleSamples=sample;
  this.particleGeometry.setDrawRange(0,sample.length);
  for(const name of ['position','kind','pointSize','opacity'])this.particleGeometry.attributes[name].needsUpdate=true;
 }
 particleSnapshot(){return {simulationTime:this.lastParticleTime,reducedMotion:this.lastParticleReduced,renderOrigin:this.lastParticleOrigin,particles:structuredClone(this.lastParticleSamples)};}
 upload(data){const g=makeChunk(data,this.assets);this.groups.add(g);this.scene.add(g);return g;}
 evict(g){this.groups.delete(g);disposeChunk(g,this.assets);}
 resize(){
  const w=Math.max(1,innerWidth),h=Math.max(1,innerHeight);this.dpr=Math.min(this.dpr,2560/w,1600/h,Math.sqrt(1800000/(w*h)));this.renderer.setPixelRatio(this.dpr);this.renderer.setSize(w,h,false);this.composer.setPixelRatio(this.dpr);this.composer.setSize(w,h);this.camera.aspect=w/h;this.camera.updateProjectionMatrix();this.fxaa.uniforms.resolution.value.set(1/(w*this.dpr),1/(h*this.dpr));
  if(this.particleMaterial)this.particleMaterial.uniforms.pixelRatio.value=this.dpr;
 }
 draw(player,yaw,pitch,time,dt,reduced){
  if(this.disposed)return;
  this.time.value=time;this.motion.value=reduced?.35:1;this.waterTime.value=time*this.motion.value;
  const y0=localRender('0',player.chunk[1],0,SIZE,512);
  this.camera.position.set(player.local[0],player.local[1]+1.68,player.local[2]);this.camera.rotation.set(pitch,Math.PI+yaw,0,'YXZ');
  this.sky.position.copy(this.camera.position);
  for(const g of this.groups){const c=g.userData.chunk;try{g.position.set(localRender(c[0],player.chunk[0],0,SIZE,64),y0,localRender(c[2],player.chunk[2],0,SIZE,64));g.visible=true;}catch{g.visible=false;}}
  this.panorama.root.position.set(this.camera.position.x,this.camera.position.y,this.camera.position.z);
  const a=authored(player);this.landmarks.root.visible=!!a;
  if(a)this.landmarks.root.position.set(-a.x+player.local[0],y0,-a.s+player.local[2]);
  this.updateParticles(player,time,reduced);
  this.sun.position.copy(this.camera.position).add(new T.Vector3(78,31,-80));this.sun.target.position.copy(this.camera.position);
  this.renderer.info.reset();this.composer.render();this.frames++;
  // Hysteresis modifies only rendering resolution, never canonical terrain/collision/identity.
  if(dt>22){this.slow++;this.fast=0;}else if(dt>0&&dt<13){this.fast++;this.slow=Math.max(0,this.slow-1);}
  if(this.slow>150&&this.dpr>.8){this.dpr=Math.max(.8,this.dpr-.2);this.slow=0;this.resize();}
  if(this.fast>900&&this.dpr<Math.min(devicePixelRatio||1,B.budgets.dpr)){this.dpr=Math.min(this.dpr+.1,devicePixelRatio||1,B.budgets.dpr);this.fast=0;this.resize();}
 }
 stats(){const gl=this.renderer.getContext(),dimensions={cameraAspect:this.camera.aspect,drawingBufferWidth:gl.drawingBufferWidth,drawingBufferHeight:gl.drawingBufferHeight};if(this.disposed)return {...dimensions,backend:'WebGL2',drawCalls:0,triangles:0,geometries:0,textures:0,particles:0,dpr:this.dpr,renderedFrames:this.frames,renderTargetBytes:0,assetTextureBytes:0,assetCpuBytes:0};return {...dimensions,backend:'WebGL2',drawCalls:this.renderer.info.render.calls,triangles:this.renderer.info.render.triangles,geometries:this.renderer.info.memory.geometries,textures:this.renderer.info.memory.textures,particles:this.particleCount,dpr:this.dpr,renderedFrames:this.frames,assetTextureBytes:Math.ceil((8*1024*1024*4+512*512*4+128*160*4)*4/3)+(this.environmentTarget?this.environmentTarget.width*this.environmentTarget.height*8:0),assetCpuBytes:58720256,renderTargetBytes:Math.ceil(this.canvas.width*this.canvas.height*32)+2048*2048*8};}
 dispose(){
  if(this.disposed)return;this.disposed=true;this.assetsReady=false;this.atmosphere.dispose();this.lastParticleSamples=[];
  for(const g of [...this.groups])this.evict(g);this.landmarks.dispose();this.panorama.dispose();this.assets.dispose();this.scene.environment=null;this.environmentTarget?.dispose();this.skyGeometry.dispose();this.skyMaterial.dispose();this.particleGeometry.dispose();this.particleMaterial.dispose();this.sun.shadow.map?.dispose();this.renderPass.dispose();this.outputPass.dispose();this.fxaa.dispose();this.composer.dispose();this.renderer.dispose();this.scene.clear();
 }
}
