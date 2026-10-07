// Keep the displayed odds and both wheel geometries in agreement.
export const CLUB_MULTIPLIERS=[5,3,2,1,.5,.1,10];
export const CLUB_CHANCES=[7,10,10,10,30,30,3];
export const CLUB_COLORS=['#8b668d','#478a83','#b58b53','#8e735b','#766b8f','#466b83','#bd715f'];
export const CLUB_STARTS=CLUB_CHANCES.map((_,i)=>CLUB_CHANCES.slice(0,i).reduce((sum,n)=>sum+n,0));
export const clubSectorCenter=sector=>(CLUB_STARTS[sector]+CLUB_CHANCES[sector]/2)/100;
