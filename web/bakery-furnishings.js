// Bakery coordinates are local to the plot, so a saved room can reopen anywhere.
export const bakeryPalette={teal:'#478b82',blue:'#607fa8',red:'#b75e50',purple:'#9177a7',sand:'#c9b88b',black:'#39403d',white:'#ece7d5'};
export function ownedBakery(snapshot){return !snapshot?.me?.island&&!snapshot?.me?.inCave?(snapshot?.nodes||[]).find(n=>n.kind==='bakery_plot'&&n.id===snapshot.me.bakeryPlot&&n.owner===snapshot.me.id):null;}
export function insideOwnBakery(snapshot){const plot=ownedBakery(snapshot),p=snapshot?.me;return plot&&Math.abs(p.x-plot.x)<=9.7&&Math.abs(p.z-plot.z)<=7.7&&Math.abs(p.y||0)<=2?plot:null;}
export function bakeryFootprint(object,catalog){const recipe=catalog?.find(r=>r.id===object.kind),angle=(object.rotation||0)*Math.PI/180,c=Math.abs(Math.cos(angle)),s=Math.abs(Math.sin(angle));return {width:(recipe?.width||1)*c+(recipe?.depth||1)*s,depth:(recipe?.depth||1)*c+(recipe?.width||1)*s,room:recipe?.room||'any'};}
export function bakeryPlacementValid(object,catalog,objects=[]){
  const entry=catalog?.find(r=>r.id===object.kind),limit=entry?.maxCount>0?entry.maxCount:entry?.core?1:Infinity;
  if(objects.filter(other=>other.id!==object.id&&other.kind===object.kind).length>=limit)return false;
  const {width,depth,room}=bakeryFootprint(object,catalog),x=object.x,z=object.z;
  if(!Number.isFinite(x)||!Number.isFinite(z)||Math.abs(x)+width/2>9.5||Math.abs(z)+depth/2>7.5)return false;
  if(room==='kitchen'&&z+depth/2> -1.35||room==='cafe'&&z-depth/2<-.65)return false;
  if(['rug','pendant'].includes(object.kind))return true;
  // Match the server's rotated footprints; connectivity is still authoritative.
  const recipe=catalog.find(r=>r.id===object.kind)||{},a=(object.rotation||0)*Math.PI/180;
  for(const doorZ of [-1,8])for(let px=-1.6;px<=1.6;px+=.4)for(let pz=doorZ-.5;pz<=doorZ+.5;pz+=.25){const dx=px-x,dz=pz-z,lx=dx*Math.cos(a)-dz*Math.sin(a),lz=dx*Math.sin(a)+dz*Math.cos(a);if(Math.hypot(Math.max(0,Math.abs(lx)-(recipe.width||1)/2),Math.max(0,Math.abs(lz)-(recipe.depth||1)/2))<.35)return false;}
  if(z-depth/2<-.88&&z+depth/2> -1.12)return false;
  return !objects.some(other=>{if(other.id===object.id||['rug','pendant'].includes(other.kind))return false;const otherRecipe=catalog.find(r=>r.id===other.kind)||{};for(const rotation of [object.rotation||0,other.rotation||0])for(const turn of [0,90]){const angle=(rotation+turn)*Math.PI/180,ux=Math.cos(angle),uz=-Math.sin(angle),project=(f,r)=>{const q=(f.rotation||0)*Math.PI/180;return Math.abs(ux*Math.cos(q)-uz*Math.sin(q))*(r.width||1)/2+Math.abs(ux*Math.sin(q)+uz*Math.cos(q))*(r.depth||1)/2;};if(Math.abs((x-other.x)*ux+(z-other.z)*uz)>=project(object,recipe)+project(other,otherRecipe)+.05)return false;}return true;});
}
export function createBakeryFurnishings({THREE,box,cylinder,sphere,flatLabel,disposeObject}){
  const plots=new Map(),raycaster=new THREE.Raycaster();let editing='';
  const remove=o=>{o.removeFromParent();disposeObject?.(o);};
  function objectView(object,catalog=[]){
    const g=new THREE.Group(),r=catalog.find(r=>r.id===object.kind)||{},w=r.width||1,d=r.depth||1,h=r.height||1,color=bakeryPalette[object.color]||'#bfa579',dark='#6f5740',steel='#7e8e89';g.name='bakery-furniture-'+(object.id||object.kind);g.userData.furniture=object;
    const b=(a,b,c,col,x,y,z)=>box(a,b,c,col,x,y,z,g),leg=(x,z,top=.76)=>b(.075,top,.075,dark,x,top/2,z);
    switch(object.kind){
      case 'oven':b(w,h,d,color,0,h/2,0);b(w*.75,h*.47,.045,'#35443f',0,h*.44,d/2+.026);b(w*.8,.07,.11,steel,0,h*.76,d/2+.08);for(const x of [-.28,0,.28])cylinder(.065,.065,.04,'#ead6a6',x*w,h*.87,d/2+.055,g,8).rotation.x=Math.PI/2;b(w+.08,.08,d+.08,steel,0,h+.04,0);cylinder(.13,.16,1.25,steel,0,h+.65,-d*.3,g,8);break;
      case 'mixer':b(w,h*.62,d,color,0,h*.31,0);b(w+.08,.08,d+.08,dark,0,h*.65,0);b(w*.42,h*.3,d*.45,steel,0,h*.84,-d*.18);cylinder(w*.3,w*.17,h*.22,'#dce1d7',0,h*.77,d*.14,g,14);b(.07,.2,.07,'#b8c8c1',0,h*.95,d*.14);break;
      case 'fridge':b(w,h,d,'#d6dfd7',0,h/2,0);b(w*.94,h*.96,.08,object.color?color:'#dce5dd',0,h/2,d/2+.04);b(w*.94,.035,.03,steel,0,h*.72,d/2+.085);for(const y of [h*.42,h*.84])b(.055,h*.2,.07,dark,-w*.3,y,d/2+.13);for(let i=0;i<5;i++)b(w*.7,.025,.03,'#9bafa5',0,.12+i*.04,d/2+.09);{const label=flatLabel('CHEF’S FRIDGE','#456259','#eff1e4',384,70);label.position.set(.1,h*.59,d/2+.09);label.scale.set(w*.67,.21,1);g.add(label);}break;
      case 'display':b(w,.95,d,color,0,.475,0);b(w+.08,.08,d+.08,dark,0,.99,0);b(w+.08,.06,d+.08,'#cee4d8',0,1.62,0);for(const x of [-w/2,w/2])b(.05,.64,.05,steel,x,1.3,-d/2);b(w,.5,.025,'#d8c39b',0,.55,d/2+.02);break;
      case 'chair':b(w,.12,d*.82,color,0,.53,0);for(const x of [-w*.37,w*.37])for(const z of [-d*.32,d*.32])leg(x,z,.48);b(w,h-.56,.1,color,0,.56+(h-.56)/2,d*.38);break;
      case 'table':b(w,.12,d,color,0,h-.06,0);for(const x of [-w*.39,w*.39])for(const z of [-d*.36,d*.36])leg(x,z,h-.12);break;
      case 'cooling_table':b(w,.1,d,'#aebdb6',0,h-.05,0);for(const x of [-w*.4,w*.4])for(const z of [-d*.35,d*.35])b(.08,h-.1,.08,color,x,(h-.1)/2,z);for(let i=0;i<9;i++)b(.025,.025,d*.86,'#647d74',-w*.43+i*w*.107,h+.014,0);b(w*.87,.025,.025,'#647d74',0,h+.014,-d*.43);b(w*.87,.025,.025,'#647d74',0,h+.014,d*.43);break;
      case 'counter':b(w,h,d,color,0,h/2,0);b(w+.07,.08,d+.07,dark,0,h+.04,0);b(w*.9,.04,.035,'#dfcca5',0,h*.62,d/2+.02);break;
      case 'bread_rack':for(const x of [-w*.45,w*.45])b(.075,h,d,dark,x,h/2,0);for(let i=0;i<4;i++)b(w,.07,d,color,0,.15+i*(h-.2)/3,0);break;
      case 'plant':cylinder(w*.29,w*.21,h*.33,color,0,h*.165,0,g,10);for(let i=0;i<6;i++){const a=i*2.4,leaf=sphere(w*.25,i%2?'#6e9253':'#4f7746',Math.cos(a)*w*.2,h*.52+i*h*.055,Math.sin(a)*d*.2,g,0);leaf.scale.set(.55,1.5,.6);}break;
      case 'pendant':cylinder(.025,.025,.7,dark,0,5.45,0,g,6);cylinder(w*.16,w*.5,.42,color,0,4.82,0,g,12);sphere(.14,'#ffe9a9',0,4.57,0,g,1);break;
      case 'rug':b(w,.025,d,color,0,.09,0);for(const x of [-w*.46,w*.46])b(.07,.026,d*.92,'#ead9b2',x,.105,0);break;
      case 'menu_board':b(w,h,.12,dark,0,h/2,0);b(w*.9,h*.88,.04,'#345349',0,h/2,.08);{const label=flatLabel('FRESH BAKES','#efdfb3',null,384,110);label.position.set(0,h*.7,.105);label.scale.set(w*.82,h*.2,1);g.add(label);}for(let i=0;i<4;i++)b(w*.58,.022,.02,'#d8cbaa',0,h*(.52-i*.085),.11);break;
      default:b(w,h,d,color,0,h/2,0);
    }
    return g;
  }
  function sync(snapshot,scenery){
    const alive=new Set();for(const plot of snapshot.nodes||[]){if(plot.kind!=='bakery_plot')continue;const building=scenery.get(plot.id);if(!building)continue;alive.add(plot.id);let record=plots.get(plot.id);if(record?.building!==building){if(record)remove(record.group);const group=new THREE.Group();group.name='bakery-furnishings';building.add(group);record={building,group,objects:new Map()};plots.set(plot.id,record);}
      const ids=new Set();for(const saved of plot.bakeryFurniture||[]){const object={...saved,color:saved.color||plot.paint?.[saved.kind]||''};ids.add(object.id);const signature=JSON.stringify(object);let view=record.objects.get(object.id);if(view?.userData.signature!==signature){if(view)remove(view);view=objectView(object,snapshot.bakeryFurnitureCatalog);view.userData.signature=signature;view.position.set(object.x,0,object.z);view.rotation.y=(object.rotation||0)*Math.PI/180;record.group.add(view);record.objects.set(object.id,view);}view.visible=object.id!==editing;}
      for(const [id,view]of record.objects)if(!ids.has(id)){remove(view);record.objects.delete(id);}
    }for(const [id,record]of plots)if(!alive.has(id)){remove(record.group);plots.delete(id);}
  }
  function setEditing(id){editing=id||'';for(const record of plots.values())for(const [key,view]of record.objects)view.visible=key!==editing;}
  function pickObject(camera,plotID,range=12){const record=plots.get(plotID);if(!record)return null;record.building.updateWorldMatrix(true,true);camera.updateMatrixWorld();raycaster.setFromCamera({x:0,y:0},camera);raycaster.far=range;const roots=record.building.children.filter(o=>o.visible&&!['bakery-custom','bakery-stock-display'].includes(o.name));for(const hit of raycaster.intersectObjects(roots,true)){let view=hit.object;while(view&&!view.userData.furniture)view=view.parent;if(view)return view.visible?{...view.userData.furniture,distance:hit.distance}:null;return null;}return null;}
  return {sync,objectView,pickObject,setEditing,dispose(){for(const record of plots.values())remove(record.group);plots.clear();}};
}
