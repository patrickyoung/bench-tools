// Navigation and forms work without JavaScript. Only active command output polls.
const active = document.querySelector('[data-job]');
if (active) {
  const status = document.getElementById('connection-note');
  const update = async () => {
    if (document.hidden) { setTimeout(update, 2000); return; }
    try {
      const response = await fetch(`/jobs/${active.dataset.job}/status`, { cache: 'no-store', signal: AbortSignal.timeout(8000) });
      if (!response.ok) throw new Error('Status unavailable');
      const result = await response.json();
      if (!result.Active) { location.reload(); return; }
      document.getElementById('job-label').textContent = result.Label;
      document.getElementById('job-note').textContent = result.Note;
      document.getElementById('stdout').textContent = result.Stdout || 'No output yet.';
      document.getElementById('stderr').textContent = result.Stderr || 'No diagnostics.';
      status.textContent = '';
    } catch {
      status.textContent = 'Live updates are unavailable. Your command may still be running. Use Refresh status to check.';
    }
    setTimeout(update, 2000);
  };
  setTimeout(update, 1000);
}

// Keep an in-progress message through a status refresh in this tab only.
const composer = document.querySelector('.conversation-composer:not(.work-composer)');
if (composer) {
  const key = `hire-draft:${location.pathname}`;
  const message = composer.querySelector('[name="message"]');
  try {
    if (!message.value) message.value = sessionStorage.getItem(key) || '';
    message.addEventListener('input', () => { try { sessionStorage.setItem(key, message.value); } catch {} });
    composer.addEventListener('submit', () => { try { sessionStorage.removeItem(key); } catch {} });
  } catch { /* Navigation and forms still work when storage is unavailable. */ }
}

// Conversation polling reports observed command state only, never invented
// reasoning or percentages. The forms remain usable without JavaScript.
const assistantProgress = document.querySelector('[data-assistant-job]');
if (assistantProgress) {
  const refreshConversation = async () => {
    if (!document.hidden) {
      try {
        const response = await fetch(`/jobs/${assistantProgress.dataset.assistantJob}/status`, {cache:'no-store', signal:AbortSignal.timeout(8000)});
        if (!response.ok) throw new Error('Status unavailable');
        const state = await response.json();
        if (!state.Active) { location.reload(); return; }
        assistantProgress.textContent = state.Label === 'Stopping' ? 'Stopping this turn…' : 'Working on this turn. The command and its evidence are available in Activity.';
      } catch { assistantProgress.textContent = 'Live status is unavailable. Your turn may still be running; check Activity before retrying.'; }
    }
    setTimeout(refreshConversation, 2000);
  };
  setTimeout(refreshConversation, 1500);
}

const workMessage=document.getElementById('work-message');
if(workMessage){
 document.querySelectorAll('[data-work-example]').forEach(button=>button.addEventListener('click',()=>{workMessage.value=button.dataset.workExample;workMessage.focus();}));
 document.querySelectorAll('[data-guide-team]').forEach(link=>link.addEventListener('click',()=>{document.getElementById('team-guidance-hint').hidden=false;workMessage.focus();}));
 const key=`hire-work-draft:${location.pathname}`;
 try{if(!workMessage.value)workMessage.value=sessionStorage.getItem(key)||'';workMessage.addEventListener('input',()=>{try{sessionStorage.setItem(key,workMessage.value)}catch{}});workMessage.form.addEventListener('submit',()=>{try{sessionStorage.removeItem(key)}catch{}})}catch{}
}
const workProgress=document.querySelector('[data-work-job]');
if(workProgress){const pollWork=async()=>{try{const response=await fetch(`/work/jobs/${workProgress.dataset.workJob}/status`,{cache:'no-store',signal:AbortSignal.timeout(8000)});if(!response.ok)throw Error();const status=await response.json();if(!status.active){const files=document.getElementById('work-attachments');if(files?.files.length){workProgress.textContent='This attempt has stopped. Your selected files are ready to send.';workMessage.form.action='/work';workMessage.form.querySelector('button[type=submit]').textContent='Send ↑';return}if(status.destination){location.assign(status.destination)}else{location.reload()}return}if(status.message)workProgress.textContent=status.message;
const specialist=document.querySelector(`[data-specialist="${workProgress.dataset.workJob}"]`);
if(specialist&&status.specialist){const data=status.specialist;specialist.hidden=!data.name;for(const key of ['name','kind','about','reason','source','access','references']){const field=specialist.querySelector(`[data-specialist-field="${key}"]`);if(field.textContent!==(data[key]||''))field.textContent=data[key]||''}const members=data.members||[];const list=specialist.querySelector('[data-specialist-members]');if(JSON.stringify(members)!==list.dataset.members){list.replaceChildren(...members.map(text=>{const item=document.createElement('li');item.textContent=text;return item}));list.dataset.members=JSON.stringify(members)}specialist.querySelector('[data-specialist-roster]').hidden=!members.length}
const expertise=document.querySelector(`[data-expertise="${workProgress.dataset.workJob}"]`);
if(expertise&&status.specialist){const note=status.specialist.expertise||{};expertise.hidden=!note.title;for(const key of ['title','why']){const field=expertise.querySelector(`[data-expertise-${key}]`);if(field.textContent!==(note[key]||''))field.textContent=note[key]||''}const members=note.members||[];const list=expertise.querySelector('[data-expertise-members]');if(list.dataset.members!==JSON.stringify(members)){list.replaceChildren(...members.map(member=>{const item=document.createElement('li');const name=document.createElement('span');name.textContent=member.name;const mode=document.createElement('small');mode.textContent=member.mode;item.append(name,mode);return item}));list.dataset.members=JSON.stringify(members)}expertise.querySelector('[data-expertise-guidance]').textContent=status.deferred?'The team is already working; your notes will guide the follow-up.':'Share a focus, style, or priority. Updates are picked up between steps.'}
const guidance=document.querySelector('[data-steering-help]');if(guidance){guidance.textContent=status.deferred?'Notes are saved for the follow-up while this team works.':'Updates are picked up between steps.'}
const update=document.querySelector(`[data-task-update="${workProgress.dataset.workJob}"]`);
if(update&&status.update){for(const key of ['done','now','next','blocked']){const row=update.querySelector(`[data-update-row="${key}"]`);const value=update.querySelector(`[data-update-value="${key}"]`);const text=status.update[key]||'';if(value.textContent!==text)value.textContent=text;row.hidden=!text}}}catch{workProgress.textContent='I may still be working. Refresh this conversation to check.'}setTimeout(pollWork,2000)};setTimeout(pollWork,1500)}

// Follow the latest exchange while leaving every earlier version accessible.
const workTurns=document.querySelectorAll('.work-turn');
if(workTurns.length>1 && !location.hash){workTurns[workTurns.length-1].scrollIntoView({block:'start'});}

// Copy is a convenience; the visible, selectable link works without scripting.
document.querySelectorAll('[data-copy-share]').forEach(button=>button.addEventListener('click',async()=>{
 const section=button.closest('.delivery-actions');const input=section.querySelector('.share-url');const status=section.querySelector('[data-copy-status]');
 try{await navigator.clipboard.writeText(input.value);status.textContent='Link copied.'}catch{input.focus();input.select();status.textContent='Link selected. Choose Copy to share it.'}
}));

const workAttachments=document.getElementById('work-attachments');
if(workAttachments){workAttachments.addEventListener('change',()=>{const files=[...workAttachments.files];const status=document.querySelector('[data-attachment-status]');let problem='';if(files.length>5)problem='Choose up to 5 files.';else if(files.some(f=>f.size>25*1024*1024))problem='Each file must be 25 MB or smaller.';else if(files.reduce((sum,f)=>sum+f.size,0)>50*1024*1024)problem='Keep these files under 50 MB in total.';workAttachments.setCustomValidity(problem);status.textContent=problem||files.map(f=>f.name).join(' · ');});}
