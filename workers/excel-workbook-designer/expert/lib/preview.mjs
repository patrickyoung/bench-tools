import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import {FileBlob,SpreadsheetFile} from '@oai/artifact-tool';
const root=path.resolve(process.argv[2]),p=path.join(root,'output/qa.json');
const qa=JSON.parse(await fs.readFile(p,'utf8'));
if(qa.zero_baseline_charts?.length||qa.preserved_conditional_fills?.length){
 const wb=await SpreadsheetFile.importXlsx(await FileBlob.load(path.join(root,'output/workbook.xlsx')));
 const spec=JSON.parse(await fs.readFile(path.join(root,'output/spec.json'),'utf8'));
 for(const preview of qa.previews){
  const range=preview.range||spec.sheets.find(s=>s.name===preview.sheet).display_range;
  const blob=await wb.render({sheetName:preview.sheet,range,scale:1.5,format:'png'});
  const bytes=new Uint8Array(await blob.arrayBuffer());await fs.writeFile(path.join(root,preview.path),bytes);
  preview.sha256=crypto.createHash('sha256').update(bytes).digest('hex');
 }
 qa.preview_basis='Saved XLSX after native feature preservation';await fs.writeFile(p,JSON.stringify(qa,null,2)+'\n');
}
