// Fixed bakery architecture. Stations and furnishings are supplied separately
// from the owner's authoritative layout, so packing one removes it completely.
import {buildFlatRoof} from './roof-scenery.js';
export const BAKERY_ROOM = Object.freeze({width:20,depth:16,height:5.8,partitionZ:-1,doorWidth:3.2,doorHeight:3.5});

export function buildBakeryRoom({THREE,parent,box,sphere,cylinder,mat,flatLabel},options={}) {
  const {width:w,depth:d,height:h,partitionZ,doorWidth:door,doorHeight}=BAKERY_ROOM;
  const wall=options.color||'#e7d9bd',trim='#597866',wood='#986746',cream='#f2e6cc';
  parent.name=options.id||'bakery-room';
  const named=(name,...args)=>{const object=box(...args,parent);object.name=name;return object;};
  const painted=(name,...args)=>{const object=named(name,...args);object.userData.paintTarget='walls';return object;};
  function floor(name,depth,z,kitchen){
    let geometry=new THREE.PlaneGeometry(w,depth,20,kitchen?7:1).toNonIndexed();
    geometry.rotateX(-Math.PI/2);
    const positions=geometry.attributes.position,colors=[];
    for(let i=0;i<positions.count;i+=6){
      const x=(positions.getX(i)+positions.getX(i+1)+positions.getX(i+2))/3;
      const localZ=(positions.getZ(i)+positions.getZ(i+1)+positions.getZ(i+2))/3;
      const alternate=(Math.floor(x+w/2)+Math.floor(localZ+depth/2))%2;
      const color=new THREE.Color(kitchen?(alternate?'#d8dfcb':'#edf0dc'):(Math.floor(x+w/2)%3?'#bf9367':'#c89f72'));
      for(let j=0;j<6;j++)colors.push(color.r,color.g,color.b);
    }
    geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));
    const object=new THREE.Mesh(geometry,mat('#ffffff',{vertexColors:true,roughness:1}));
    object.name=name;object.position.set(0,.08,z);object.receiveShadow=true;parent.add(object);
  }
  floor('kitchen-floor',7,-4.5,true);floor('cafe-floor',9,3.5,false);
  painted('bakery-back',w,h,.24,wall,0,h/2,-d/2);
  for(const side of [-1,1]){
    painted('bakery-side-'+side,.24,h,d,wall,side*w/2,h/2,0);
    named('bakery-wainscot-'+side,.08,1.15,d-.3,trim,side*(w/2-.16),.6,0);
    named('bakery-side-rail-'+side,.12,.1,d-.3,cream,side*(w/2-.19),1.22,0);
  }
  const wing=(w-door)/2;
  for(const side of [-1,1]){
    const x=side*(door/2+wing/2),windowWidth=6.2;
    // Front windows are real glazed openings; the opaque sill, piers and
    // lintel stay aligned with the server's solid front-wall footprint.
    painted('bakery-front-sill-'+side,wing,1.15,.24,wall,x,.575,d/2);
    painted('bakery-front-top-'+side,wing,h-4.35,.24,wall,x,4.35+(h-4.35)/2,d/2);
    const pier=(wing-windowWidth)/2;
    for(const edge of [-1,1])painted('bakery-window-pier-'+side+'-'+edge,pier,3.2,.24,wall,x+edge*(windowWidth/2+pier/2),2.75,d/2);
    const glass=named('bakery-window-'+side,windowWidth,3.2,.08,'#a8c4bb',x,2.75,d/2);
    glass.material=mat('#aecdc3',{transparent:true,opacity:.22,roughness:.1,metalness:.05,depthWrite:false});glass.castShadow=false;
    for(const edge of [-1,1])named('bakery-window-frame-'+side+'-'+edge,.14,3.35,.32,trim,x+edge*windowWidth/2,2.75,d/2+.025);
    for(const y of [1.15,4.35])named('bakery-window-rail-'+side+'-'+y,windowWidth+.2,.14,.32,trim,x,y,d/2+.025);
    named('bakery-window-mullion-'+side,.1,3.2,.16,cream,x,2.75,d/2+.04);
    named('bakery-window-crossbar-'+side,windowWidth,.08,.16,cream,x,2.9,d/2+.04);
    painted('bakery-partition-'+side,wing,h,.24,wall,x,h/2,partitionZ);
    named('bakery-partition-panel-'+side,wing-.12,1.15,.08,trim,x,.6,partitionZ+.16);
    named('bakery-partition-rail-'+side,wing,.1,.12,cream,x,1.22,partitionZ+.18);
    for(const z of [partitionZ,d/2])named('bakery-door-jamb-'+side+'-'+z,.14,doorHeight,.36,wood,side*(door/2+.075),doorHeight/2,z);
  }
  for(const [name,z] of [['entrance',d/2],['partition',partitionZ]]){
    painted('bakery-'+name+'-lintel',door,h-doorHeight,.24,wall,0,doorHeight+(h-doorHeight)/2,z);
    named('bakery-'+name+'-door-trim',door+.35,.16,.38,wood,0,doorHeight+.08,z);
  }
  buildFlatRoof({box,parent},{width:w,depth:d,wallTop:h,color:'#897c63',edgeColor:trim,name:'bakery-ceiling'});
  const awning=named('bakery-awning',12,.12,1.35,'#c9865f',0,4.6,d/2+.65);awning.rotation.x=.09;
  for(let i=0;i<12;i++)named('bakery-awning-stripe-'+i,.45,.035,1.36,'#f2e3c3',-5.5+i,4.67,d/2+.65).rotation.x=.09;
  const sign=flatLabel(options.label||'BAKERY · AVAILABLE','#5d4935','#f3e7ce',768,90);
  sign.name='bakery-sign';sign.position.set(0,5.08,d/2+.17);sign.scale.set(12,.78,1);parent.add(sign);parent.userData.sign=sign;
  const kitchen=flatLabel('THE KITCHEN','#4c6958','#e9ead5',512,90);kitchen.name='bakery-kitchen-sign';kitchen.position.set(0,4.15,partitionZ+.14);kitchen.scale.set(4,.6,1);parent.add(kitchen);
  return parent;
}
