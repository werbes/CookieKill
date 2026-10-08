// Small held bouquets share the world palette and need no downloaded assets.
export function createBouquetView({THREE,sphere,box,cylinder},item){
  const kind={flower_bouquet:'mixed',bouquet_daisies:'daisy',bouquet_tulips:'tulip',bouquet_lavender:'lavender'}[item];if(!kind)return null;
  const root=new THREE.Group();root.name='held-bouquet-'+item;root.userData.bouquetKind=kind;
  const wrap={mixed:'#d3b785',daisy:'#e6cb8f',tulip:'#d79cab',lavender:'#aa9fc5'}[kind],ribbon={mixed:'#b77984',daisy:'#a3884e',tulip:'#a45577',lavender:'#716893'}[kind];
  cylinder(.2,.045,.31,wrap,0,-.055,0,root,7);cylinder(.063,.06,.07,ribbon,0,-.15,0,root,8);
  for(const side of [-1,1]){const bow=sphere(.075,ribbon,side*.06,-.145,-.075,root,1);bow.scale.set(1,.55,.3);}
  const count=kind==='lavender'?7:5;
  for(let i=0;i<count;i++){
    const angle=i*Math.PI*2/count,x=Math.sin(angle)*.14,z=Math.cos(angle)*.11,y=.26+(i%2)*.08,flowerKind=kind==='mixed'?['daisy','tulip','lavender','daisy','tulip'][i]:kind;
    const stem=cylinder(.012,.015,y+.13,'#60885b',x/2,y/2-.04,z/2,root,5);stem.rotation.z=-x*.7;stem.rotation.x=z*.7;
    const leaf=sphere(.08,'#779961',x,.14,z,root,1);leaf.scale.set(.35,1,.2);leaf.rotation.z=x>0?-.55:.55;
    const bloom=new THREE.Group();bloom.name='bouquet-flower-'+flowerKind;bloom.position.set(x,y,z);root.add(bloom);
    if(flowerKind==='daisy'){
      for(let j=0;j<7;j++){const a=j*Math.PI*2/7,petal=sphere(.055,'#fff0ce',Math.sin(a)*.054,.006,Math.cos(a)*.054,bloom,1);petal.scale.set(.7,.26,1);petal.rotation.y=a;}
      sphere(.037,'#e6b758',0,.02,0,bloom,1).scale.y=.5;
    }else if(flowerKind==='tulip'){
      for(let j=0;j<4;j++){const a=j*Math.PI/2,petal=sphere(.061,j%2?'#cf769e':'#e597b2',Math.sin(a)*.028,.035,Math.cos(a)*.028,bloom,1);petal.scale.set(.68,1.22,.68);petal.rotation.z=Math.sin(a)*-.18;}
    }else{
      for(let j=0;j<6;j++){const bud=sphere(.027,j%2?'#a88dce':'#c2a4dd',Math.sin(j*2.4)*.025,j*.035,Math.cos(j*2.4)*.025,bloom,1);bud.scale.y=1.3;}
    }
  }
  root.rotation.set(-.2,0,.14);root.position.y=.02;return root;
}
