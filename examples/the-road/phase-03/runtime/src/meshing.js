import * as T from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {B,R,L} from './config.js';
import {center,profile,terrain,at,streamS,waterLevel,cross,support} from './generation.js';
export {Assets} from './materials.js';
const linearColor=hex=>new T.Color(hex);
function whiteColors(g){const a=new Float32Array(g.attributes.position.count*3);a.fill(1);g.setAttribute('color',new T.BufferAttribute(a,3));return g;}
function surface(data,material){
 const g=new T.BufferGeometry();g.setAttribute('position',new T.BufferAttribute(data.p,3));g.setAttribute('color',new T.BufferAttribute(data.c,3));g.setAttribute('uv',new T.BufferAttribute(data.uv,2));g.setIndex(new T.BufferAttribute(data.index,1));g.computeVertexNormals();g.computeBoundingSphere();
 const mesh=new T.Mesh(g,material);mesh.receiveShadow=true;mesh.userData.ownedGeometry=true;return mesh;
}
export function makeChunk(data,assets){
 const group=new T.Group();group.userData.chunk=data.chunk;group.userData.kind=data.kind;group.userData.shared=[];
 group.add(surface(data.ground,assets.materials.ground));assets.retain('ground');group.userData.shared.push('ground');
 for(const r of data.roads){group.add(surface(r,assets.materials.road));assets.retain('road');group.userData.shared.push('road');}
 const obj=new T.Object3D(),col=new T.Color();
 for(const [kind,b] of Object.entries(data.batches)){
  if(!b.ids.length)continue;
  const geometry=kind==='wood'?assets.geometries.wood.clone():assets.geometries[kind];
  if(kind==='wood')geometry.setAttribute('branchTaper',new T.InstancedBufferAttribute(b.tapers,1));
  const mesh=new T.InstancedMesh(geometry,assets.materials[kind],b.ids.length);
  if(kind==='wood'){mesh.customDepthMaterial=assets.depthWood;mesh.userData.ownedGeometry=true;}
  for(let i=0;i<b.ids.length;i++){const a=i*12;obj.position.fromArray(b.data,a);obj.scale.fromArray(b.data,a+3);obj.rotation.set(b.data[a+6],b.data[a+7],b.data[a+8],'YXZ');obj.updateMatrix();mesh.setMatrixAt(i,obj.matrix);col.fromArray(b.data,a+9);mesh.setColorAt(i,col);}
  mesh.instanceMatrix.needsUpdate=true;if(mesh.instanceColor)mesh.instanceColor.needsUpdate=true;mesh.computeBoundingBox();mesh.computeBoundingSphere();
  mesh.castShadow=kind!=='grass'&&kind!=='flowers'&&kind!=='litter';mesh.receiveShadow=true;mesh.userData.ids=b.ids;mesh.userData.shared=true;
  assets.retain(kind);group.userData.shared.push(kind);group.add(mesh);
 }
 return group;
}
export function disposeChunk(group,assets){
 group.removeFromParent();group.traverse(o=>{if(o.userData.ownedGeometry)o.geometry.dispose();if(o.isInstancedMesh)o.dispose();});
 for(const k of group.userData.shared)assets.release(k);
 group.clear();
}
function colorGeometry(g,color){whiteColors(g);const c=linearColor(color),a=g.attributes.color;for(let i=0;i<a.count;i++)a.setXYZ(i,c.r,c.g,c.b);return g;}
export function makeLandmarks(assets){
 const root=new T.Group(),owned=[],mats=[],geos=[],far=new T.Group(),near=new T.Group();root.add(far,near);
 const groups=new Map();
 const add=(geometry,kind,color,position,rotation=[0,0,0],parent=near)=>{
  if(geometry.index){const original=geometry;geometry=geometry.toNonIndexed();original.dispose();}colorGeometry(geometry,color);
  geometry.rotateX(rotation[0]);geometry.rotateY(rotation[1]);geometry.rotateZ(rotation[2]);geometry.translate(...position);
  if(kind==='stone'||kind==='wood'){
   const p=geometry.attributes.position,n=geometry.attributes.normal,uv=geometry.attributes.uv;
   for(let i=0;i<p.count;i++){
    const ax=Math.abs(n.getX(i)),ay=Math.abs(n.getY(i)),az=Math.abs(n.getZ(i));
    uv.setXY(i,(ay>ax&&ay>az?p.getX(i):ax>az?p.getZ(i):p.getX(i))*.5,(ay>ax&&ay>az?p.getZ(i):p.getY(i))*.5);
   }
  }
  const key=(parent===far?'far:':'near:')+kind;if(!groups.has(key))groups.set(key,{kind,parent,parts:[]});groups.get(key).parts.push(geometry);
 };
 const box=(kind,color,x,y,z,sx,sy,sz,rot=[0,0,0],parent=near)=>add(new T.BoxGeometry(sx,sy,sz),kind,color,[x,y,z],rot,parent);
 const stone=B.palette.stone,wood='#75654e',earth=B.palette.earth;
 // Trodden oval and old fire ring at the great oak, on the land rather than a gameplay trigger.
 const ox=center(R.oakStation)+6.7,oy=terrain(at(ox,R.oakStation));
 for(let i=0;i<13;i++){const a=i*Math.PI*2/13,x=ox-2.7+Math.cos(a)*.88,z=R.oakStation-3+Math.sin(a)*.88;const g=new T.SphereGeometry(1,8,5);g.scale(.22,.14,.19);add(g,'stone',i%3?stone:'#575951',[x,terrain(at(x,z))+.04,z],[0,a,0]);}
 // Three deliberately different hedge openings.
 for(let i=0;i<R.gaps.length;i++){
  const s=R.gaps[i],x=center(s)+6.4,y=terrain(at(x,s));
  if(i!==1)for(const dz of [-1.85,1.85])box('wood',wood,x,y+.8,s+dz,.18,1.6,.18);
  if(i===0){box('wood',wood,x,y+.65,s,.7,.13,1.1);for(const h of [.5,1,1.35])box('wood',wood,x,y+h,s,.1,.1,3.5);}
  if(i===1){for(const dz of [-1.5,1.5])box('stone',stone,x,y+.35,s+dz,.55,.7,.7);box('wood',wood,x,y+1.25,s,.16,.15,3.5);}
  if(i===2){for(const h of [.45,.9,1.3])box('wood',wood,x,y+h,s,.12,.10,3.6,[.04,0,0]);box('wood',wood,x,y+.83,s,.14,.12,3.5,[.35,0,0]);}
 }
 // Low stone bridge: two masonry arch faces and rough coping, clear inside width >= road width.
 const bs=R.bridgeStation,bx=center(bs),deck=profile(bs),span=R.bridgeSpan,inner=R.width/2+.04;
 for(const side of [-1,1]){
  const x=bx+side*(inner+.20),start=-span/2-1.8,end=span/2+1.8;
  for(let row=0;row<10;row++){
   const low=waterLevel()-.4+row*.29;
   let z=start-(row%2)*.35,index=0;
   while(z<end){
    const width=.66+.18*(.5+.5*Math.sin(index*9.17+row*4.6+side));
    const za=Math.max(start,z),zb=Math.min(end,z+width-.025),mid=(za+zb)/2,top=profile(bs+mid)+R.parapetHeight-.13;
    const arch=waterLevel()-.25+1.32*Math.sqrt(Math.max(0,1-(mid/(span/2))**2));
    const floor=Math.abs(mid)<span/2?arch:waterLevel()-.4;
    const bottom=Math.max(low,floor),high=Math.min(low+.273,top);
    if(high>bottom&&zb>za)box('stone',['#9fa18e','#8e9582','#a7a796','#959a86'][(row+index)%4],x,(bottom+high)/2,bs+mid,.40,high-bottom,zb-za,[0,.009*Math.sin(index*3),0]);
    z+=width;index++;
   }
  }
  for(let z=start;z<end;z+=.91){
   const width=Math.min(.89,end-z),s=bs+z+width/2,top=profile(s)+R.parapetHeight;
   box('stone','#a5a797',x,top-.09,s,.47,.18,width,[0,0,.009*Math.sin(z*13)]);
  }
 }
 box('stone','#979b8b',bx,deck-.12,bs,R.width+.08,.20,span);
 // Stream ribbon follows the same canonical channel. Shader time is attached by renderer.
 const wp=[],wu=[],wi=[];
 for(let i=0;i<=240;i++){const x=-480+i*4;for(const side of [-1,1]){wp.push(x,waterLevel(),streamS(x)+side*span/2*(1-(Math.abs(x-bx)>8?.055*(1+Math.sin(x*.67)*Math.cos(x*.23)):0)));wu.push(i/12,(side+1)/2);}if(i<240){const a=i*2;wi.push(a,a+1,a+2,a+1,a+3,a+2);}}
 const wg=new T.BufferGeometry();wg.setAttribute('position',new T.Float32BufferAttribute(wp,3));wg.setAttribute('uv',new T.Float32BufferAttribute(wu,2));wg.setIndex(wi);wg.computeVertexNormals();
 const waterMat=new T.MeshPhysicalMaterial({color:'#827759',roughness:L.materials.water.roughness,metalness:0,transparent:true,opacity:L.materials.water.opacity,depthWrite:false,side:T.DoubleSide,clearcoat:L.materials.water.clearcoat,clearcoatRoughness:.28,ior:1.333,envMapIntensity:.85});
 const water=new T.Mesh(wg,waterMat);water.receiveShadow=true;near.add(water);owned.push(water);mats.push(waterMat);geos.push(wg);
 // Leaning milestone with a tiny worn inscription rendered on locally generated canvas.
 const ms=R.milestoneStation,mx=center(ms)+4.55,my=terrain(at(mx,ms));
 const mg=new T.CylinderGeometry(.27,.38,1.28,8);add(mg,'stone','#979989',[mx,my+.6,ms],[.05,0,-.14]);
 const cv=document.createElement('canvas');cv.width=128;cv.height=160;const ctx=cv.getContext('2d');ctx.fillStyle='#999b8b';ctx.fillRect(0,0,128,160);ctx.fillStyle='rgba(65,66,57,.33)';ctx.font='26px Georgia';ctx.textAlign='center';ctx.fillText('THE',64,57);ctx.fillText('ROAD',64,91);ctx.font='20px Georgia';ctx.fillText('I',64,126);
 const inscription=new T.CanvasTexture(cv);inscription.colorSpace=T.SRGBColorSpace;
 const im=new T.MeshStandardMaterial({map:inscription,roughness:.98}),ig=new T.PlaneGeometry(.43,.63),ins=new T.Mesh(ig,im);
 ins.position.set(mx-.02,my+.75,ms-.27);ins.rotation.z=-.14;near.add(ins);mats.push(im);geos.push(ig);owned.push(ins);
 // Far lane, fields and hedge-lines read from arrival and reverse view. Near lane overrides by 5 cm.
 for(let s=-1800;s<R.length+700;s+=8){
  const x=center(s),x2=center(s+8),h=profile(s)-.05,h2=profile(s+8)-.05;
  const g=new T.BufferGeometry();g.setAttribute('position',new T.Float32BufferAttribute([x-3.048,h,s,x2-3.048,h2,s+8,x+3.048,h,s,x+3.048,h,s,x2-3.048,h2,s+8,x2+3.048,h2,s+8],3));g.setAttribute('uv',new T.Float32BufferAttribute([0,0,1,0,0,1,1,0,1,1,0,1],2));g.computeVertexNormals();add(g,'road',B.palette.dust,[0,0,0],[0,0,0],far);
 }
 // Distant village: pitched roofs, varied chimneys, no implied interiors.
 for(let i=0;i<14;i++){
  const x=(i%5-2)*17+Math.sin(i*7)*7,s=R.villageStation+Math.floor(i/5)*22+Math.sin(i*3)*8,y=terrain(at(x,s)),h=4+i%3;
  box('stone','#b3afa0',x,y+h/2,s,9,h,7,[0,0,0],far);
  const roof=new T.BufferGeometry(),v=[[-4.9,0,-3.9],[4.9,0,-3.9],[0,2.7,-3.9],[-4.9,0,3.9],[4.9,0,3.9],[0,2.7,3.9]],p=[],uv=[];
  for(const face of [[0,2,1],[3,4,5],[0,3,5],[0,5,2],[1,2,5],[1,5,4]])for(const j of face){p.push(...v[j]);uv.push(v[j][0]/2,v[j][2]/2);}
  roof.setAttribute('position',new T.Float32BufferAttribute(p,3));roof.setAttribute('uv',new T.Float32BufferAttribute(uv,2));roof.computeVertexNormals();
  add(roof,'stone',i%2?'#555750':'#79604b',[x,y+h,s],[0,0,0],far);
  box('stone','#80796b',x+2,y+h+2.5,s,.8,3,.8,[0,0,0],far);
 }
 // Thin meandering field divisions follow actual terrain instead of floating screen bars.
 for(let row=0;row<7;row++)for(let i=0;i<110;i++){
  const x=(i-55)*7,s=1400+row*67+Math.sin(i*.10+row)*20;
  const h1=terrain(at(x-3.5,s)),h2=terrain(at(x+3.5,s)),height=.43+.22*(.5+.5*Math.sin(i*2.7+row));
  box('ground','#62775d',x,(h1+h2)/2+height/2-.05,s,7.08,height,.58,[0,.018,Math.atan2(h2-h1,7)],far);
 }
 for(const {kind,parent,parts} of groups.values()){
  const g=mergeGeometries(parts);parts.forEach(p=>p.dispose());const m=new T.Mesh(g,assets.materials[kind]);m.castShadow=parent===near;m.receiveShadow=true;parent.add(m);geos.push(g);assets.retain(kind);
 }
 return {root,water,near,far,dispose(){root.removeFromParent();for(const g of geos)g.dispose();for(const m of mats)m.dispose();inscription.dispose();for(const {kind} of groups.values())assets.release(kind);root.clear();}};
}
