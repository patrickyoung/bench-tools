import {generate,buffersOf} from './generation.js';
import {B} from './config.js';
self.onmessage=({data:job})=>{
 try{
  if(job.seed!==B.seed||job.version!==B.generationVersion)throw Error('Generation identity mismatch');
  const payload=generate(job),buffers=buffersOf(payload);
  self.postMessage({type:'result',id:job.id,chunk:job.chunk,seed:job.seed,version:job.version,epoch:job.epoch,revision:job.revision,payload},buffers);
  // Ownership really moved: this is checked in the worker, not assumed.
  if(buffers.some(b=>b.byteLength!==0))throw Error('Transfer did not detach');
 }catch(error){self.postMessage({type:'job-error',id:job.id,message:String(error)});}
};
self.postMessage({type:'ready'});
