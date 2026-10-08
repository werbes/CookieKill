import {achievementPaintingDataURL} from './achievement-painting-art.js';

// Head-local cosmetics: the same hat rig is used by players and bakery staff.
export function buildAchievementHat({THREE,parent,box,sphere,cylinder},id){
  if(!id||id==='none')return null;
  const root=new THREE.Group();root.name='achievement-hat-'+id;root.position.y=1.9;
  const brim=(color,r=.34)=>cylinder(r,r,.055,color,0,0,0,root,14);
  const dome=(color,r=.27)=>{const d=sphere(r,color,0,.08,0,root);d.scale.y=.65;return d;};
  const band=(color,r=.255,y=.12)=>cylinder(r,r,.065,color,0,y,0,root,14);
  const crown=color=>{band(color,.285,.085);for(let i=0;i<6;i++){const a=i*Math.PI/3;cylinder(0,.07,.23,color,Math.cos(a)*.24,.23,Math.sin(a)*.24,root,4);}};
  switch(id){
    case 'chef':case 'master_chef':{
      const tall=id==='master_chef';cylinder(.25,.25,tall?.4:.25,'#f6edda',0,tall?.19:.08,0,root,12);
      for(const x of [-.18,0,.18])sphere(.2,'#fff7e8',x,tall?.47:.26,0,root);
      if(tall){band('#b4747c',.255,.045);box(.065,.105,.025,'#d6b55f',0,.1,-.26,root);}break;
    }
    case 'builder':brim('#ddb148');dome('#e7bd53');box(.07,.07,.49,'#f4ce76',0,.21,0,root);break;
    case 'champion':crown('#dfbd60');break;
    case 'ocean':crown('#73adc1');for(const x of [-.14,0,.14])sphere(.045,'#cce9dc',x,.2,-.245,root,1);break;
    case 'recycler':brim('#688f62',.39);cylinder(.21,.29,.23,'#7fa373',0,.13,0,root,10);band('#466d4d',.275,.045);break;
    case 'green_guardian':{
      const b=dome('#548a62',.32);b.rotation.z=.18;
      for(let i=0;i<3;i++){const leaf=sphere(.13,'#91b87c',-.18+i*.035,.28+i*.035,0,root,1);leaf.scale.set(.35,1.1,.35);leaf.rotation.z=-.5+i*.45;}break;
    }
    case 'marine_warden':brim('#3e5e78');cylinder(.31,.25,.2,'#f0eee0',0,.15,0,root,10);band('#536fa2',.258,.07);box(.065,.12,.03,'#dabc64',0,.15,-.266,root);box(.16,.04,.03,'#dabc64',0,.12,-.266,root);break;
    case 'rancher':brim('#b48454',.48);cylinder(.23,.29,.3,'#caa16d',0,.16,0,root,8);band('#795b42',.285,.07);for(const side of [-1,1]){const edge=box(.13,.07,.48,'#b48454',side*.4,.045,0,root);edge.rotation.z=side*.3;}break;
    case 'racer':dome('#c85d4d',.31);box(.42,.14,.09,'#394e56',0,.08,-.28,root);for(const x of [-.12,.12])box(.17,.085,.035,'#c5e3dc',x,.09,-.335,root);box(.065,.06,.53,'#f3e4b7',0,.26,0,root);break;
    case 'trailblazer':brim('#9a9471',.43);cylinder(.25,.31,.26,'#b7ad84',0,.13,0,root,10);band('#747e58',.295,.055);cylinder(.035,.035,.17,'#ebd8ab',-.29,.13,-.05,root,6);break;
    case 'merchant':brim('#695b87',.39);cylinder(.245,.26,.48,'#87739e',0,.25,0,root,12);band('#d2b26a',.264,.07);box(.1,.1,.03,'#e8cd81',0,.1,-.27,root);break;
    case 'architect':dome('#7199a6');box(.39,.045,.25,'#96b9bf',0,.015,-.29,root);box(.23,.09,.035,'#edf0d6',0,.11,-.255,root);for(const x of [-.07,0,.07])box(.025,.06,.025,'#608995',x,.11,-.28,root);break;
    case 'restaurateur':{const b=dome('#936f83',.32);b.rotation.z=-.23;cylinder(.035,.035,.12,'#cfa6af',.09,.32,0,root,6);break;}
    case 'artisan':brim('#d4b87d',.42);cylinder(.29,.3,.2,'#e2ca96',0,.13,0,root,12);band('#9c6668',.304,.07);for(let i=0;i<5;i++){const a=i*Math.PI*2/5;sphere(.055,'#ddb2a2',.28+Math.sin(a)*.045,.15+Math.cos(a)*.045,-.16,root,1);}break;
    default:return null;
  }
  parent?.add(root);return root;
}

export function createAchievementScenery({THREE,parent,box,cylinder,flatLabel,disposeObject}){
  const root=new THREE.Group();root.name='bakery-achievement-galleries';parent.add(root);const galleries=new Map(),paintings=new Map();
  const remove=object=>{object.traverse(part=>{const id=part.userData.achievementPaintingId;if(!id)return;part.material.dispose();if(!disposeObject)part.geometry.dispose();const art=paintings.get(id);if(art&&--art.references===0){art.texture.dispose();paintings.delete(id);}});object.removeFromParent();disposeObject?.(object);};
  function painting(id){
    let art=paintings.get(id);if(!art){const texture=typeof document==='undefined'?new THREE.Texture():new THREE.TextureLoader().load(achievementPaintingDataURL(id));texture.colorSpace=THREE.SRGBColorSpace;texture.userData.achievementPaintingId=id;art={texture,references:0};paintings.set(id,art);}art.references++;
    const picture=new THREE.Mesh(new THREE.PlaneGeometry(1.02,.57375),new THREE.MeshBasicMaterial({map:art.texture}));picture.name='achievement-painting-'+id;picture.userData.achievementPaintingId=id;picture.position.set(0,.112,.102);return picture;
  }
  function sync(snapshot){
    root.visible=!snapshot?.me?.island;const catalog=new Map((snapshot?.achievementCatalog||[]).map(a=>[a.id,a])),present=new Set();
    for(const node of snapshot?.nodes||[]){
      if(node.kind!=='bakery_plot'||!node.owner)continue;
      const ids=[...new Set(node.hungAchievements||[])].filter(id=>catalog.has(id)).slice(0,15);if(!ids.length)continue;
      present.add(node.id);const signature=JSON.stringify([node.owner,node.x,node.z,ids]),old=galleries.get(node.id);if(old?.signature===signature)continue;if(old)remove(old.object);
      const gallery=new THREE.Group();gallery.name='achievement-gallery-'+node.id;gallery.position.set(node.x,0,node.z);root.add(gallery);
      ids.forEach((id,i)=>{
        const achievement=catalog.get(id),frame=new THREE.Group();frame.name='hung-achievement-'+id;frame.position.set(9.73,2+(Math.floor(i/5))*1.12,.45+(i%5)*1.44);frame.rotation.y=-Math.PI/2;gallery.add(frame);
        box(1.2,.94,.11,'#94774f',0,0,0,frame);box(1.08,.82,.035,'#f1e6c9',0,0,.07,frame);
        for(const y of [-.4375,.4375])box(1.2,.065,.055,'#b99a64',0,y,.07,frame);
        for(const x of [-.5675,.5675])box(.065,.81,.055,'#b99a64',x,0,.07,frame);
        frame.add(painting(id));
        const name=flatLabel(achievement.name,'#665039',null,512,90);name.position.set(0,-.25,.12);name.scale.set(1.02,.13,1);frame.add(name);
        const bonus=flatLabel('+10% original price','#7e7759',null,384,70);bonus.position.set(0,-.365,.12);bonus.scale.set(.9,.105,1);frame.add(bonus);
      });
      galleries.set(node.id,{object:gallery,signature});
    }
    for(const [id,gallery]of galleries)if(!present.has(id)){remove(gallery.object);galleries.delete(id);}
  }
  return {root,sync,dispose(){for(const g of galleries.values())remove(g.object);galleries.clear();root.removeFromParent();}};
}
