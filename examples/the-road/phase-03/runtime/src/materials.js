import * as T from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {L} from './config.js';
import {RoundedBoxGeometry} from 'three/addons/geometries/RoundedBoxGeometry.js';
const url=file=>new URL(`./assets/${file}`,import.meta.url).href;
function random(seed=9871){let a=seed;return()=>{a=(Math.imul(a,1664525)+1013904223)>>>0;return a/4294967296;};}
function white(g){const c=new Float32Array(g.attributes.position.count*3);c.fill(1);g.setAttribute('color',new T.BufferAttribute(c,3));return g;}
function sprigTexture(){
 const canvas=document.createElement('canvas');canvas.width=canvas.height=512;
 const ctx=canvas.getContext('2d'),rand=random(3461);
 ctx.clearRect(0,0,512,512);
 // A leafy twig, not a giant solid polygon. Transparent interstices survive depth/shadow tests.
 ctx.strokeStyle='rgba(121,115,83,.9)';ctx.lineWidth=3;ctx.beginPath();ctx.moveTo(244,490);ctx.bezierCurveTo(225,330,291,181,256,28);ctx.stroke();
 for(let i=0;i<25;i++){
  const y=52+i*16,x=255+Math.sin(i*.37)*15,side=i%2?1:-1;
  const cx=x+side*(44+rand()*90),cy=y-16-rand()*20;
  ctx.lineWidth=1.6;ctx.beginPath();ctx.moveTo(x,y+15);ctx.quadraticCurveTo((cx+x)/2,y-7,cx,cy);ctx.stroke();
  ctx.save();ctx.translate(cx,cy);ctx.rotate(side*(.8+rand()*.7));
  const length=25+rand()*17,width=10+rand()*7,shade=Math.floor(182+rand()*65);
  ctx.fillStyle=`rgb(${shade},${Math.min(255,shade+7)},${Math.max(0,shade-15)})`;
  ctx.beginPath();ctx.moveTo(0,length);
  ctx.bezierCurveTo(-width*1.25,length*.3,-width*.86,-length*.46,0,-length);
  ctx.bezierCurveTo(width*1.13,-length*.3,width*1.02,length*.4,0,length);ctx.fill();
  ctx.strokeStyle='rgba(111,120,78,.34)';ctx.lineWidth=.9;ctx.beginPath();ctx.moveTo(0,length*.8);ctx.lineTo(0,-length*.82);ctx.stroke();ctx.restore();
 }
 const map=new T.CanvasTexture(canvas);map.colorSpace=T.SRGBColorSpace;map.anisotropy=4;
 return map;
}
function leafCluster(){
 const rand=random(421),parts=[];
 for(let i=0;i<38;i++){
  const a=rand()*Math.PI*2,z=rand()*2-1,r=Math.cbrt(rand())*.93,s=Math.sqrt(1-z*z);
  const g=new T.PlaneGeometry(.65+rand()*.35,.69+rand()*.30,2,2),p=g.attributes.position;
  for(let j=0;j<p.count;j++)p.setZ(j,.11*(p.getX(j)*p.getX(j)-p.getY(j)*.3));
  g.computeVertexNormals();g.rotateX((rand()-.5)*2.8);g.rotateY(a);g.rotateZ(rand()*1.4-.7);
  g.translate(Math.cos(a)*s*r,z*r,Math.sin(a)*s*r);parts.push(g.toNonIndexed());g.dispose();
 }
 const g=white(mergeGeometries(parts));parts.forEach(p=>p.dispose());
 const c=g.attributes.color,p=g.attributes.position;
 for(let i=0;i<c.count;i++){const v=.9+.1*(p.getY(i)+1)/2;c.setXYZ(i,v,v,v*.96);}
 return g;
}
function grassGeometry(){
 const p=[],uv=[],rand=random(121);
 for(let i=0;i<19;i++){
  const a=rand()*6.28,x=(rand()-.5)*.34,z=(rand()-.5)*.34,h=.35+rand()*.45,w=(.014+rand()*.018)/3;
  const dx=Math.cos(a)*w,dz=Math.sin(a)*w,bx=Math.sin(a)*.09,bz=Math.cos(a)*.09;
  const v=[[x-dx,0,z-dz],[x+dx,0,z+dz],[x+bx-dx*.6,h*.55,z+bz-dz*.6],[x+bx+dx*.6,h*.55,z+bz+dz*.6],[x+bx*1.8,h,z+bz*1.8]];
  for(const ids of [[0,2,1],[1,2,3],[2,4,3]])for(const j of ids){p.push(...v[j]);uv.push(j%2,j/4);}
 }
 const g=new T.BufferGeometry();g.setAttribute('position',new T.Float32BufferAttribute(p,3));g.setAttribute('uv',new T.Float32BufferAttribute(uv,2));g.computeVertexNormals();return white(g);
}
function crookedWood(){
 const g=new T.CylinderGeometry(1,1,1,13,9,true),p=g.attributes.position;
 for(let i=0;i<p.count;i++){
  const y=p.getY(i),t=y+.5;
  const bend=.085*Math.sin(t*Math.PI)*Math.sin(t*4.1);
  const grain=1+.025*Math.sin(Math.atan2(p.getZ(i),p.getX(i))*7+Math.sin(t*Math.PI)*1.5);
  p.setXYZ(i,p.getX(i)*grain+bend,y,p.getZ(i)*grain+Math.sin(t*Math.PI)*.025);
 }
 g.computeVertexNormals();return white(g);
}
export function wornPaving(variant){
 const g=new RoundedBoxGeometry(2,2,2,2,.17+variant*.035),p=g.attributes.position;
 for(let i=0;i<p.count;i++){
  const x=p.getX(i),y=p.getY(i),z=p.getZ(i);
  p.setXYZ(i,x*(.91+.055*Math.sin(z*3.2+variant*2.1)),y*(.94+.035*Math.sin(x*3.7+z*4.3+variant)),z*(.92+.05*Math.cos(x*4.1-variant*1.8)));
 }
 g.computeVertexNormals();return white(g);
}
export function taperShader(shader){
 shader.vertexShader=shader.vertexShader.replace('#include <common>','#include <common>\n#ifdef USE_INSTANCING\nattribute float branchTaper;\n#endif')
  .replace('#include <begin_vertex>','#include <begin_vertex>\n#ifdef USE_INSTANCING\ntransformed.xz*=mix(1.,branchTaper,clamp(position.y+.5,0.,1.));\n#endif')
  .replace('#include <beginnormal_vertex>','#include <beginnormal_vertex>\n#ifdef USE_INSTANCING\nobjectNormal.y+=1.-branchTaper;\n#endif');
}
function scannedMaterial(roughness,normalScale){
 return new T.MeshStandardMaterial({color:0xffffff,vertexColors:true,roughness,metalness:0,normalScale:new T.Vector2(normalScale,normalScale),envMapIntensity:.55});
}
function scanShader(material,kind,soil){
 material.onBeforeCompile=shader=>{
  if(soil)shader.uniforms.soilScan={value:soil};
  const soilDecl=soil?'uniform sampler2D soilScan;':'';
  shader.fragmentShader=shader.fragmentShader.replace('#include <common>',`#include <common>\n${soilDecl}`);
  shader.fragmentShader=shader.fragmentShader.replace('#include <map_fragment>',`
   #ifdef USE_MAP
    vec4 scanned=texture2D(map,vMapUv);
    ${soil?'float bare=smoothstep(1.43,1.73,vColor.r/max(.001,vColor.g));scanned=mix(scanned,texture2D(soilScan,vMapUv),bare);':''}
    float lum=dot(scanned.rgb,vec3(.2126,.7152,.0722));
    vec3 chroma=clamp(scanned.rgb/max(.06,lum),vec3(.6),vec3(1.4));
    diffuseColor.rgb*=(${kind==='road'?'.76+.85':'.52+1.25'}*lum)*mix(vec3(1.),chroma,${kind==='road'?'.12':'.32'});
   #endif`);
  if(kind==='wood'){
   taperShader(shader);
   shader.vertexShader=shader.vertexShader.replace('#include <uv_vertex>',`#include <uv_vertex>
    #if defined(USE_INSTANCING) && defined(USE_MAP) && defined(USE_NORMALMAP)
     vec2 metres=vec2(length(instanceMatrix[0].xyz)*3.14159,length(instanceMatrix[1].xyz)*.5);
     vMapUv*=metres;vNormalMapUv*=metres;
    #endif`);
  }
 };
 material.customProgramCacheKey=()=>`road-scan-${kind}-v2`;
}
export class Assets{
 constructor(){
  this.disposed=false;this.refs=new Map();this.textures={leaf:sprigTexture()};
  this.materials={
   ground:scannedMaterial(L.materials.earth.roughness,.55),
   road:scannedMaterial(L.materials.dust.roughness,.16),
   wood:scannedMaterial(L.materials.bark.roughness,.65),
   stone:scannedMaterial(L.materials.stone.roughness,.5),
   foliage:new T.MeshStandardMaterial({color:0xffffff,vertexColors:true,map:this.textures.leaf,alphaTest:.18,side:T.DoubleSide,shadowSide:T.DoubleSide,roughness:.98,metalness:0,envMapIntensity:.65,emissive:'#717650',emissiveIntensity:.13}),
   grass:new T.MeshStandardMaterial({color:'#dddcc8',vertexColors:true,side:T.DoubleSide,roughness:1,metalness:0,envMapIntensity:.55}),
   flowers:new T.MeshStandardMaterial({color:0xffffff,vertexColors:true,map:this.textures.leaf,alphaTest:.46,side:T.DoubleSide,roughness:1,metalness:0}),
   litter:scannedMaterial(.98,.15)
  };
  this.geometries={foliage:leafCluster(),wood:crookedWood(),stone:white(new T.SphereGeometry(1,11,7)),grass:grassGeometry(),flowers:leafCluster(),litter:white(new T.SphereGeometry(1,6,3))};
  for(let i=0;i<3;i++){this.geometries['paving'+i]=wornPaving(i);this.materials['paving'+i]=this.materials.stone;}
  this.depthWood=new T.MeshDepthMaterial({depthPacking:T.RGBADepthPacking});this.depthWood.onBeforeCompile=taperShader;this.depthWood.customProgramCacheKey=()=>'taper-depth-v1';
  const loader=new T.TextureLoader();
  const load=async(name,file,color)=>{
   const tex=await loader.loadAsync(url(file));
   tex.colorSpace=color?T.SRGBColorSpace:T.NoColorSpace;tex.wrapS=tex.wrapT=T.RepeatWrapping;tex.anisotropy=8;
   if(this.disposed){tex.dispose();return null;}this.textures[name]=tex;return tex;
  };
  this.ready=Promise.all(['brown_mud','bark_willow','mossy_rock','grass_ground'].flatMap(name=>[
   load(name,`${name}_diff_1k.jpg`,true),load(name+'_normal',`${name}_nor_gl_1k.jpg`,false)
  ])).then(()=>{
   if(this.disposed)return;
   for(const [kind,scan] of [['ground','grass_ground'],['road','brown_mud'],['wood','bark_willow'],['stone','mossy_rock'],['litter','brown_mud']]){
    const m=this.materials[kind];m.map=this.textures[scan];m.normalMap=this.textures[scan+'_normal'];scanShader(m,kind,kind==='ground'?this.textures.brown_mud:null);m.needsUpdate=true;
   }
  });
 }
 retain(key){this.refs.set(key,(this.refs.get(key)||0)+1);}
 release(key){this.refs.set(key,Math.max(0,(this.refs.get(key)||0)-1));}
 dispose(){if(this.disposed)return;this.disposed=true;for(const t of Object.values(this.textures))t.dispose();for(const g of Object.values(this.geometries))g.dispose();for(const m of new Set(Object.values(this.materials)))m.dispose();this.depthWood.dispose();this.refs.clear();}
}
