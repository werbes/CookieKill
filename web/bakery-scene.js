// Customer and staff nodes come from the same server simulation that owns sales.
export const BAKERY_ACTOR_KINDS=['bakery_customer','bakery_chef','bakery_cashier'];
export function bakeryDisplayItems(stock,limit=12){
  const counts=Object.entries(stock||{}).filter(([,n])=>Number.isInteger(n)&&n>0).sort(([a],[b])=>a.localeCompare(b));
  const items=[];while(items.length<limit&&counts.some(([,n])=>n>0)){for(const entry of counts){if(entry[1]>0&&items.length<limit){items.push(entry[0]);entry[1]--;}}}return items;
}
export function createBakeryScene({THREE,parent,makePerson,makeCookie,box,cylinder,flatLabel,disposeObject}){
  const group=new THREE.Group();group.name='bakery-people';parent.add(group);
  const actors=new Map(),displays=new Map(),remove=object=>{object.removeFromParent();disposeObject?.(object);};
  const hash=id=>Array.from(id).reduce((sum,c)=>(sum*31+c.charCodeAt(0))>>>0,0);
  function actor(node){
    const object=new THREE.Group(),chef=node.kind==='bakery_chef',cashier=node.kind==='bakery_cashier',seed=hash(node.id);
    const avatar={skin:seed%5,shirt:'tee',shirtColor:chef?'white':cashier?'teal':['sand','purple','blue','red','teal'][seed%5],pantsColor:chef?'black':'sand',hat:chef?'chef':''};
    const body=makePerson(chef?'Chef':cashier?'Cashier':'',chef?'#efe6ce':'#7c9380',false,avatar);object.add(body);
    if(chef)for(const child of body.children)if(child.isSprite)child.position.y=2.65;
    // The general character builder includes a sample cookie. Customers arrive
    // empty-handed and only carry a purchase once their real order is served.
    for(const child of [...body.children])if(child.isGroup&&Math.abs(child.position.x-.37)<.001&&Math.abs(child.position.y-.97)<.001)remove(child);
    const legs=[],arms=[];
    for(const side of [-1,1]){
      const leg=new THREE.Group();leg.position.set(side*.16,.71,0);body.add(leg);legs.push(leg);
      const arm=new THREE.Group();arm.position.set(side*.35,1.37,0);body.add(arm);arms.push(arm);
      for(const child of [...body.children]){
        if(!child.isMesh)continue;
        let pivot=null;
        if(Math.abs(child.position.x-side*.16)<.025&&child.position.y<.76)pivot=leg;
        else if(Math.abs(child.position.x-side*.35)<.045&&child.position.y>.75&&child.position.y<1.45)pivot=arm;
        if(pivot){child.position.sub(pivot.position);pivot.add(child);}
      }
    }
    if(chef||cashier){box(.44,.48,.035,chef?'#ece4ca':'#d5bf99',0,1.01,-.277,body);box(.15,.22,.035,chef?'#ece4ca':'#d5bf99',0,1.33,-.27,body);box(.2,.11,.025,'#b4a68b',0,.99,-.302,body);}
    const purchase=new THREE.Group();purchase.name='customer-purchase';box(.25,.3,.19,'#b89768',0,0,0,purchase);box(.28,.035,.2,'#d4bc91',0,.15,0,purchase);purchase.position.set(.03,-.43,-.09);arms[1].add(purchase);purchase.visible=false;
    object.position.set(node.x,node.y||0,node.z);object.rotation.y=node.yaw||0;object.userData={node,body,legs,arms,purchase,seed,target:new THREE.Vector3(node.x,node.y||0,node.z)};group.add(object);return object;
  }
  function stockDisplay(node,building,hasCashier,furniture){
    let record=displays.get(node.plotId);const signature=JSON.stringify([node.owner,node.stock||{},hasCashier,furniture]);
    if(record?.building===building&&record.signature===signature)return;
    if(record)remove(record.object);
    const object=new THREE.Group();object.name='bakery-stock-display';object.position.set(furniture.x,0,furniture.z);object.rotation.y=(furniture.rotation||0)*Math.PI/180;object.visible=furniture.id!==snapshotEditing;building.add(object);
    const stock=node.stock||{},items=bakeryDisplayItems(stock),count=Object.values(stock).reduce((sum,n)=>sum+n,0);
    items.forEach((item,i)=>{const cookie=makeCookie(item,.38);cookie.name='stocked-'+item;cookie.position.set(-1+(i%6)*.4,1.1,-.16+Math.floor(i/6)*.3);object.add(cookie);});
    const label=flatLabel(count?count+' fresh bakes':'DISPLAY EMPTY','#674d35','#eee0be',384,70);label.position.set(0,.58,.425);label.scale.set(2.25,.32,1);object.add(label);
    if(hasCashier){const till=new THREE.Group();till.name='bakery-till';till.position.set(.97,1.74,.05);box(.43,.2,.4,'#456964',0,0,0,till);const screen=box(.3,.23,.055,'#b8d6bb',0,.18,-.1,till);screen.rotation.x=-.25;for(let i=0;i<6;i++)box(.045,.018,.045,'#e4d9b4',(i%3-1)*.075,.113,.02+Math.floor(i/3)*.075,till);object.add(till);}
    record={building,object,signature};displays.set(node.plotId,record);
  }
  let snapshotEditing='';
  function sync(snapshot,scenery){
    snapshotEditing=snapshot.me?.editingBakeryFurnitureId||'';
    group.visible=!snapshot.me?.island&&!snapshot.me?.inCave;
    const nodes=snapshot.nodes||[],alive=new Set();
    for(const node of nodes){if(!BAKERY_ACTOR_KINDS.includes(node.kind))continue;alive.add(node.id);let object=actors.get(node.id);if(!object){object=actor(node);actors.set(node.id,object);}object.userData.node=node;object.userData.target.set(node.x,node.y||0,node.z);object.userData.purchase.visible=node.kind==='bakery_customer'&&node.phase==='leaving'&&node.amount>0;}
    for(const [id,object] of actors)if(!alive.has(id)){remove(object);actors.delete(id);}
    const activeDisplays=new Set();
    for(const node of nodes){if(node.kind!=='bakery_display'||!node.owner||node.available===false)continue;const building=scenery.get(node.plotId),furniture=nodes.find(n=>n.id===node.plotId)?.bakeryFurniture?.find(o=>o.kind==='display');if(!building||!furniture)continue;activeDisplays.add(node.plotId);stockDisplay(node,building,nodes.some(n=>n.kind==='bakery_cashier'&&n.plotId===node.plotId),furniture);displays.get(node.plotId).object.visible=furniture.id!==snapshotEditing;}
    for(const [id,record] of displays)if(!activeDisplays.has(id)){remove(record.object);displays.delete(id);}
  }
  function update(time,dt){
    if(!group.visible)return;
    const blend=1-Math.exp(-Math.max(0,Math.min(dt,.1))*14);
    for(const object of actors.values()){
      const {node,body,legs,arms,target,seed}=object.userData,distance=object.position.distanceTo(target),walking=distance>.018;
      object.position.lerp(target,blend);let turn=(node.yaw||0)-object.rotation.y;turn=Math.atan2(Math.sin(turn),Math.cos(turn));object.rotation.y+=turn*blend;
      const stride=walking?Math.sin(time*8+seed)*.5:0;legs[0].rotation.x=stride;legs[1].rotation.x=-stride;arms[0].rotation.x=-stride*.7;arms[1].rotation.x=stride*.7;
      const cooking=node.kind==='bakery_chef'&&/^(mixing|baking)\b/i.test(node.phase);
      if(cooking){arms[0].rotation.x=-.55+Math.sin(time*4)*.17;arms[1].rotation.x=-.55-Math.sin(time*4)*.17;}
      body.position.y=walking?Math.abs(Math.sin(time*8+seed))*.035:0;
    }
  }
  function dispose(){for(const object of actors.values())remove(object);for(const record of displays.values())remove(record.object);actors.clear();displays.clear();group.removeFromParent();}
  return {group,sync,update,dispose};
}
