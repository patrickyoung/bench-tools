#!/usr/bin/env python3
"""Native subprocess/export foundation adapted from Bench Inkscape Illustrator (MIT).
Never executes a workspace program. HOME is preserved; profile/cache are isolated.
"""
import json, math, os, pathlib, re, shutil, signal, subprocess, time
from svg_contract import fail
TIMEOUT=120

def executable():
    selected=os.environ.get("INKSCAPE")
    if selected:
        p=pathlib.Path(selected)
        if not p.is_absolute() or not p.is_file() or not os.access(p,os.X_OK):
            fail("INKSCAPE must select an absolute executable")
        return str(p.resolve())
    found=shutil.which("inkscape")
    if not found: fail("Inkscape unavailable; unfinished, not a semantic conflict")
    return str(pathlib.Path(found).resolve())

class Native:
    def __init__(self,temp):
        self.exe=executable(); self.temp=temp
        self.records=[]; self.logs=[]
        self.env=os.environ.copy()
        for name in ("profile","cache","config","data","tmp"):
            (temp/name).mkdir()
        self.env.update({"INKSCAPE_PROFILE_DIR":str(temp/"profile"),"XDG_CACHE_HOME":str(temp/"cache"),
          "XDG_CONFIG_HOME":str(temp/"config"),"XDG_DATA_HOME":str(temp/"data"),"TMPDIR":str(temp/"tmp")})
        self.version=""
    def run(self,args,label):
        argv=[self.exe,*map(str,args)]
        started=time.monotonic(); timed=False
        proc=subprocess.Popen(argv,cwd=self.temp,env=self.env,stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE,start_new_session=True)
        try: out,err=proc.communicate(timeout=TIMEOUT)
        except subprocess.TimeoutExpired:
            timed=True
            try: os.killpg(proc.pid,signal.SIGKILL)
            except ProcessLookupError: pass
            out,err=proc.communicate()
        stdout=out.decode("utf-8","replace"); stderr=err.decode("utf-8","replace")
        self.records.append({"label":label,"argv":argv,"exit_code":proc.returncode,
                             "timed_out":timed,"duration_seconds":round(time.monotonic()-started,6)})
        self.logs.extend(["$ "+json.dumps(argv),stdout,stderr,"exit="+str(proc.returncode)+" timeout="+str(timed)])
        if timed or proc.returncode: fail("native "+label+" failed; job unfinished")
        return stdout,stderr
    def probe(self):
        version,_=self.run(["--version"],"version")
        m=re.search(r"Inkscape\s+(\d+)\.(\d+)",version)
        if not m or tuple(map(int,m.groups()))<(1,2): fail("Inkscape >=1.2 required")
        self.version=version.strip()
        out,err=self.run(["--help"],"help")
        for flag in ("--export-type","--export-plain-svg","--export-text-to-path",
                     "--export-area-page","--export-filename","--export-width","--export-height","--query-all"):
            if flag not in out+err: fail("installed help lacks "+flag)
    def export(self,source,target,label):
        self.run(["--export-type=svg","--export-plain-svg","--export-text-to-path",
                  "--export-area-page","--export-filename",target,source],label)
    def render(self,source,target,w,h,label):
        self.run(["--export-type=png","--export-area-page","--export-width",w,
                  "--export-height",h,"--export-filename",target,source],label)
    def query(self,source,label,w,h):
        out,_=self.run(["--query-all",source],label)
        boxes={}
        for line in out.splitlines():
            if not line.strip(): continue
            parts=line.rsplit(",",4)
            if len(parts)!=5 or not parts[0] or parts[0] in boxes: fail("invalid/duplicate native query row")
            try: b=[float(v) for v in parts[1:]]
            except ValueError: fail("invalid native query number")
            if not all(math.isfinite(v) for v in b) or b[2]<0 or b[3]<0: fail("invalid native bounds")
            if b[0]<-.1 or b[1]<-.1 or b[0]+b[2]>w+.1 or b[1]+b[3]>h+.1:
                fail("native object outside page: "+parts[0]+" "+str(b))
            boxes[parts[0]]=b
        if not boxes: fail("empty native query")
        return boxes
