export function assetsAvailable(state){
 return state.assetsReady===true && state.view?.assetsReady===true && !state.view?.disposed &&
  !state.loading&&!state.disposed&&!state.contextLost&&!state.terminal;
}
