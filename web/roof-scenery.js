// A level, rectangular cap whose underside rests exactly on the wall tops.
// The modest, even overhang avoids the old rotated pyramid's oversized corners.
export function buildFlatRoof({box,parent},{width,depth,wallTop,color='#79735c',edgeColor=color,name='flat-roof'}) {
  const thickness=.36,overhang=.18;
  const roof=box(width+overhang*2,thickness,depth+overhang*2,color,0,wallTop+thickness/2,0,parent);
  roof.name=name;roof.userData.flatRoof={width,depth,wallTop,thickness,overhang};
  for(const side of [-1,1]){
    const front=box(width+overhang*2,.085,.035,edgeColor,0,wallTop+.105,side*(depth/2+overhang),parent);
    front.name=name+'-edge';
    const edge=box(.035,.085,depth+overhang*2,edgeColor,side*(width/2+overhang),wallTop+.105,0,parent);
    edge.name=name+'-edge';
  }
  return roof;
}
