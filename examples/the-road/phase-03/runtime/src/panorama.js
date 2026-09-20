import * as T from 'three';

// Painted optical-distance scenery, not collision terrain. Translation follows the
// eye (including altitude); rotation stays world-aligned. Village/hedges are NOT
// children. Profiles use integer harmonics, and the closing sample is copied
// exactly rather than relying on sin(2π) rounding.
export const PANORAMA_CAPS=Object.freeze({layers:4,segments:1024,vertices:8200,draws:4});
export function makePanorama(){
 const root=new T.Group();root.name='painted-360-panorama';
 const colors=['#788d7c','#889c95','#99adb1','#a3b3bd'];
 const owned=[];
 for(let layer=0;layer<4;layer++){
  const radius=4200+layer*1000,positions=[],colorsOut=[],indices=[];
  const base=new T.Color(colors[layer]),rock=new T.Color('#909faa');
  for(let i=0;i<1024;i++){
   const a=i/1024*Math.PI*2;
   // Forward valley leaves the bare crown visible; low undulations continue
   // on both sides and behind, without constant-height bridging bands.
   const forward=Math.exp(7*(Math.cos(a-.06)-1));
   let elevation=.014+layer*.005
    +(.005+.0005*layer)*Math.sin(5*a+layer*1.7)
    +.003*Math.cos(9*a-layer)
    +.0015*Math.sin(17*a+layer*.8)
    -[.010,.008,.004,0][layer]*forward;
   const crown=Math.exp(500*(Math.cos(a-.06)-1));
   if(layer===3)elevation+=crown*(.025+.0015*Math.sin(71*a)+.001*Math.cos(113*a));
   const x=radius*Math.sin(a),z=radius*Math.cos(a);
   positions.push(x,-1200,z,x,radius*elevation,z);
   const tint=base.clone().lerp(rock,layer===3?crown:0);
   colorsOut.push(base.r,base.g,base.b,tint.r,tint.g,tint.b);
  }
  positions.push(...positions.slice(0,6));colorsOut.push(...colorsOut.slice(0,6));
  for(let i=0;i<1024;i++){const j=i*2;indices.push(j,j+1,j+2,j+1,j+3,j+2);}
  const geometry=new T.BufferGeometry();
  geometry.setAttribute('position',new T.Float32BufferAttribute(positions,3));
  geometry.setAttribute('color',new T.Float32BufferAttribute(colorsOut,3));
  geometry.setIndex(indices);geometry.computeBoundingSphere();
  const material=new T.MeshBasicMaterial({color:0xffffff,vertexColors:true,side:T.DoubleSide,fog:false});
  const mesh=new T.Mesh(geometry,material);mesh.renderOrder=-1;
  root.add(mesh);owned.push({geometry,material});
 }
 return {root,dispose(){root.removeFromParent();for(const {geometry,material} of owned){geometry.dispose();material.dispose();}root.clear();}};
}
