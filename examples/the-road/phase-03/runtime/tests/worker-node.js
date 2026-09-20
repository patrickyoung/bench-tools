import {parentPort} from 'node:worker_threads';
globalThis.self={postMessage:(data,transfer)=>parentPort.postMessage(data,transfer)};
await import('../src/chunk-worker.js');
parentPort.on('message',data=>self.onmessage({data}));
