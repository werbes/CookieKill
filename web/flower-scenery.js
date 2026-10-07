// Small botanical geometry shares the normal scenery batches: no texture loads
// or per-flower animation, and each region has its own planted character.
export function createFlowerScenery({THREE,mesh,sphere,cylinder,box,mat}){
  const colors=['#f6e3ad','#ab8dce','#e7756a','#77acd1','#e6af50','#d68db1'];
  function flower(parent,x,y,z,index=0,scale=1){
    const g=new THREE.Group();g.position.set(x,y,z);g.scale.setScalar(scale);parent.add(g);
    const variant=index%6,h=.48+(index%4)*.09,color=colors[variant];
    cylinder(.012,.021,h,'#4d774a',0,h/2,0,g,4);
    for(const side of [-1,1]){const leaf=sphere(.11,'#64894e',side*.085,h*.4,0,g);leaf.scale.set(1.5,.2,.5);leaf.rotation.z=side*.42;}
    if(variant===1){
      for(let i=0;i<4;i++)for(let j=0;j<3;j++){const a=j*Math.PI*2/3+i*.8,b=sphere(.085,color,Math.cos(a)*(.09-i*.012),h+i*.095,Math.sin(a)*(.09-i*.012),g);b.scale.set(1,.8,1);}
    }else{
      const petals=variant===2?5:6;
      for(let i=0;i<petals;i++){const a=i*Math.PI*2/petals,p=sphere(variant===2?.11:.085,color,Math.cos(a)*.095,h,Math.sin(a)*.095,g);p.scale.set(1.25,.32,.85);p.rotation.y=-a;p.rotation.z=variant===2?.22:0;}
      const center=sphere(.054,variant===2?'#563d30':'#dcb13d',0,h+.023,0,g);center.scale.y=.5;
    }
    return g;
  }
  function patch(parent,x,z,index=0,scale=1){
    const g=new THREE.Group();g.position.set(x,0,z);parent.add(g);
    for(let i=0;i<4;i++){const a=i*2.4+index;flower(g,Math.cos(a)*(.2+i*.09),0,Math.sin(a)*(.2+i*.09),index+i%2,scale*(.8+(i%3)*.13));}
    return g;
  }
  function cactus(parent,variant){
    for(const [i,p] of [[0,2.48,0],[.77,2.08,0],...(variant%2?[[-.85,2.18,0]]:[])].entries()){
      const color=['#ee82a3','#f0b954','#f0db94'][((variant+i)%3+3)%3];
      for(let j=0;j<7;j++){const a=j*Math.PI*2/7,petal=sphere(.085,color,p[0]+Math.cos(a)*.09,p[1],p[2]+Math.sin(a)*.09,parent);petal.scale.set(1.1,.5,.8);petal.rotation.y=-a;}
      sphere(.06,'#de9843',p[0],p[1]+.035,p[2],parent);
    }
  }
  function bed(parent,x,z,index=0,width=2.5,depth=.85){
    const g=new THREE.Group();g.position.set(x,0,z);parent.add(g);
    box(width,.14,depth,'#736248',0,.07,0,g);
    for(const sign of [-1,1])box(width+.13,.15,.09,'#b49c7c',0,.105,sign*(depth/2+.02),g);
    for(const sign of [-1,1])box(.09,.15,depth,'#b49c7c',sign*(width/2+.02),.105,0,g);
    for(let i=0;i<10;i++)flower(g,-width*.42+(i%5)*width*.21,.15,(i<5?-.2:.2)*depth,index+i%3,.8+(i%2)*.18);
    return g;
  }
  return {flower,patch,cactus,bed};
}
