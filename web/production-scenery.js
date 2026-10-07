// Station decorations reflect server jobs and output only; they never imply stock.
export function createProductionScene({THREE,parent,makeDough,makeCookie,flatLabel,disposeObject}){
  const home=new THREE.Group();home.name='home-production';parent.add(home);const records=new Map();
  const remove=object=>{object.removeFromParent();object.traverse(child=>{if(child.name==='production-status'){for(const material of Array.isArray(child.material)?child.material:[child.material]){material?.map?.dispose();material?.dispose();}}});disposeObject?.(object);};
  function sync(snapshot,scenery){
    home.visible=!!snapshot.me?.island;const active=new Set(),stations=[];
    for(const node of snapshot.nodes||[]){if(!['bakery_mixer','bakery_oven','bakery_cooling_table'].includes(node.kind)||!node.owner||node.available===false)continue;const building=scenery.get(node.plotId),plot=snapshot.nodes.find(n=>n.id===node.plotId),kind=node.kind.slice(7),furniture=plot?.bakeryFurniture?.find(o=>o.kind===kind);if(!building||!furniture)continue;stations.push({id:node.id,kind,parent:building,x:furniture.x,y:0,z:furniture.z,rotation:furniture.rotation||0,state:node.production||{},hidden:furniture.id===snapshot.me?.editingBakeryFurnitureId,width:kind==='cooling_table'?2.4:1.6,depth:kind==='cooling_table'?1.2:1.6,top:kind==='cooling_table'?.96:1.65});}
    for(const object of snapshot.island?.objects||[]){if(!['mixer','oven','table'].includes(object.kind))continue;const production=snapshot.me?.production?.['home:'+object.id];if(!production)continue;const r=snapshot.buildCatalog?.find(r=>r.id===object.kind)||{};stations.push({id:'home:'+object.id,kind:object.kind==='table'?'cooling_table':object.kind,parent:home,x:object.x,y:object.y||0,z:object.z,rotation:object.rotation||0,state:production,hidden:object.id===snapshot.me?.editingBuildId,width:r.width||2,depth:r.depth||1.2,top:object.kind==='table'?(r.height||1)+.08:1.5});}
    for(const station of stations){active.add(station.id);const {state}=station,job=state.job,output=Object.entries(state.output||{}).filter(([,n])=>n>0),count=output.reduce((sum,[,n])=>sum+n,0),signature=JSON.stringify([station.kind,job&&[job.item,job.amount,Math.ceil(job.remaining),job.total],output,station.x,station.y,station.z,station.rotation]);let record=records.get(station.id);
      if(record?.parent!==station.parent||record?.signature!==signature){if(record)remove(record.object);const object=new THREE.Group();object.name='production-'+station.id;object.position.set(station.x,station.y,station.z);object.rotation.y=station.rotation*Math.PI/180;station.parent.add(object);
        const items=[];for(const [item,n]of output){const take=Math.min(n,12-items.length);for(let i=0;i<take;i++)items.push(item);}
        if(station.kind==='cooling_table')items.forEach((item,i)=>{const view=makeCookie(item,.34);view.name='finished-'+item;view.position.set((i%6-2.5)*Math.min(.35,(station.width-.3)/6),station.top,(Math.floor(i/6)-.5)*.35);object.add(view);});
        if(station.kind==='mixer'&&(job||count)){const item=job?.item||output[0][0],view=makeDough(item,.8);view.name='mixing-dough';view.position.set(0,station.top,.2);object.add(view);}
        if(job||count){const text=job?(station.kind==='oven'?'BAKING':'MIXING')+' · '+Math.ceil(Math.max(0,job.remaining||0))+'s':count+' READY',label=flatLabel(text,'#5b664b','#eee4c6',384,70);label.name='production-status';label.position.set(0,station.kind==='cooling_table'?1.25:2.25,station.depth/2+.08);label.scale.set(1.8,.33,1);object.add(label);}
        record={object,parent:station.parent,signature,job:!!job};records.set(station.id,record);
      }
      record.object.visible=!station.hidden;
    }
    for(const [id,record]of records)if(!active.has(id)){remove(record.object);records.delete(id);}
  }
  function update(time){for(const record of records.values())if(record.job){const dough=record.object.getObjectByName('mixing-dough');if(dough)dough.rotation.y=time*2.5;}}
  return {sync,update,home,dispose(){for(const record of records.values())remove(record.object);records.clear();home.removeFromParent();}};
}
