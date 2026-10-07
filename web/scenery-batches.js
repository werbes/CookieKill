// Combine opaque, stationary plants into spatial chunks. Resource roots retain
// their identity; chopping only changes their triangles in the shared index.
// Moving objects, buildings, textured signs, water and lights stay independent.
export function createSceneryBatches(THREE, parent, roots, chunkSize=32) {
  parent.updateWorldMatrix(true,true);
  const inverse=new THREE.Matrix4().copy(parent.matrixWorld).invert();
  const buckets=new Map(),resources=new Map(),batches=[];
  const position=new THREE.Vector3(),normal=new THREE.Vector3(),normalMatrix=new THREE.Matrix3();
  for(const root of roots){
    const meshes=[];root.traverse(object=>{if(object.isMesh)meshes.push(object);});
    for(const object of meshes){
      const material=object.material,geometry=object.geometry;
      if(!material?.isMeshStandardMaterial||material.transparent||material.map||material.normalMap||material.emissiveMap||material.alphaMap||material.roughnessMap||material.metalnessMap||material.aoMap||material.lightMap||material.displacementMap||material.wireframe||material.opacity!==1)continue;
      const positions=geometry.getAttribute('position'),normals=geometry.getAttribute('normal'),colors=geometry.getAttribute('color');
      if(!positions||!normals||geometry.drawRange.start!==0||Number.isFinite(geometry.drawRange.count))continue;
      const transform=new THREE.Matrix4().multiplyMatrices(inverse,object.matrixWorld);
      position.setFromMatrixPosition(transform);
      const key=JSON.stringify([Math.floor(position.x/chunkSize),Math.floor(position.z/chunkSize),material.roughness,material.metalness,material.flatShading,material.side,material.emissive.getHex(),material.emissiveIntensity,object.castShadow,object.receiveShadow]);
      let bucket=buckets.get(key);
      if(!bucket){
        const shared=material.clone();shared.color.set(0xffffff);shared.vertexColors=true;
        bucket={material:shared,positions:[],normals:[],colors:[],indices:[],ranges:[],castShadow:object.castShadow,receiveShadow:object.receiveShadow};buckets.set(key,bucket);
      }
      const offset=bucket.positions.length/3,start=bucket.indices.length;
      normalMatrix.getNormalMatrix(transform);
      for(let i=0;i<positions.count;i++){
        position.fromBufferAttribute(positions,i).applyMatrix4(transform);normal.fromBufferAttribute(normals,i).applyMatrix3(normalMatrix).normalize();
        bucket.positions.push(position.x,position.y,position.z);bucket.normals.push(normal.x,normal.y,normal.z);
        bucket.colors.push(material.color.r*(colors?.getX(i)??1),material.color.g*(colors?.getY(i)??1),material.color.b*(colors?.getZ(i)??1));
      }
      const index=geometry.index,count=index?.count??positions.count;
      for(let i=0;i<count;i++)bucket.indices.push(offset+(index?index.getX(i):i));
      bucket.ranges.push({root,start,count});
      object.removeFromParent();geometry.dispose();
    }
  }
  for(const bucket of buckets.values()){
    const geometry=new THREE.BufferGeometry();
    geometry.setAttribute('position',new THREE.Float32BufferAttribute(bucket.positions,3));
    geometry.setAttribute('normal',new THREE.Float32BufferAttribute(bucket.normals,3));
    geometry.setAttribute('color',new THREE.Float32BufferAttribute(bucket.colors,3));
    geometry.setIndex(bucket.indices);geometry.index.setUsage(THREE.DynamicDrawUsage);
    geometry.computeBoundingBox();geometry.computeBoundingSphere();
    const batch=new THREE.Mesh(geometry,bucket.material);batch.name='scenery-batch';batch.castShadow=bucket.castShadow;batch.receiveShadow=bucket.receiveShadow;batch.matrixAutoUpdate=false;parent.add(batch);batches.push(batch);
    const original=geometry.index.array.slice();
    for(const range of bucket.ranges){
      let resource=resources.get(range.root);if(!resource){resource={visible:true,ranges:[]};resources.set(range.root,resource);}
      resource.ranges.push({...range,index:geometry.index,original});
    }
  }
  const setVisible=(root,visible)=>{
    const resource=resources.get(root);if(!resource||resource.visible===visible)return;
    resource.visible=visible;
    for(const {index,original,start,count} of resource.ranges){
      if(visible)index.array.set(original.subarray(start,start+count),start);
      else index.array.fill(0,start,start+count);
      index.addUpdateRange(start,count);index.needsUpdate=true;
    }
  };
  for(const root of roots)if(!root.visible)setVisible(root,false);
  return {batches,setVisible,dispose(){for(const batch of batches){batch.removeFromParent();batch.geometry.dispose();batch.material.dispose();}resources.clear();}};
}
