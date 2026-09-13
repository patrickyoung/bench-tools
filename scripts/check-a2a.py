#!/usr/bin/env python3
"""Offline public executable checks: A2A -> Tend -> Agent, without model spend."""
import argparse
from contextlib import contextmanager
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("bench_integration", ROOT / "scripts/check-integration.py")
harness = importlib.util.module_from_spec(spec)
spec.loader.exec_module(harness)
require, invoke = harness.require, harness.invoke


@contextmanager
def listener(bins, work, env, state, worker, options=()):
    card = work / "card.json"
    card.write_text(json.dumps({"name":"offline", "description":"local executable fixture", "version":"1",
                               "skills":[{"id":"fixture","name":"Fixture","description":"exercise streams","tags":["test"]}]}))
    args = [bins / "a2aserve", "-dev-loopback", "-listen", "127.0.0.1:0", "-state", state,
            "-tend", bins / "tend", *options, card, "--", *worker]
    process = subprocess.Popen(list(map(str,args)), cwd=work, env=env, stdin=subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    try:
        with selectors.DefaultSelector() as ready:
            ready.register(process.stderr, selectors.EVENT_READ)
            require(ready.select(15), "A2A listener did not report startup")
            line = process.stderr.readline().decode()
        require(" card " in line, "A2A startup failed: "+line)
        endpoint = line.split(" card ",1)[1].strip().split("/.well-known/")[0]
        yield process, endpoint
    finally:
        if process.poll() is None:
            process.terminate()
        try:
            stdout, stderr = process.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            stdout, stderr = process.communicate(timeout=5)
            raise RuntimeError("A2A listener did not stop promptly")
        require(not stdout, "Listener wrote non-protocol startup material to stdout")


def request(text, immediate=False, task=None):
    message = {"messageId":os.urandom(16).hex(),"role":"ROLE_USER","parts":[{"text":text}]}
    if task:
        message.update(taskId=task["id"],contextId=task["contextId"])
    return {"message":message,"configuration":{"returnImmediately":immediate}}


def call(bins, work, env, endpoint, method, data, code=0, verb="request"):
    result = invoke([bins / "a2a",verb,"-http-loopback","-timeout","15s",method,endpoint+"/rpc"],
                    cwd=work,env=env,data=json.dumps(data).encode(),code=code,timeout=20)
    require(not result.stderr or code != 0, "Successful result mixed diagnostics with its protocol output")
    if verb == "listen":
        return [json.loads(line) for line in result.stdout.splitlines()]
    return json.loads(result.stdout)


def wait_for(path, timeout=5):
    until = time.monotonic()+timeout
    while time.monotonic()<until:
        if path.exists():
            return
        time.sleep(.02)
    raise RuntimeError("Worker did not produce "+str(path))


def workspace(state, task):
    return state / "workspaces" / hashlib.sha256(task["id"].encode()).hexdigest() / "work"


def gone(pid, timeout=5):
    until=time.monotonic()+timeout
    while time.monotonic()<until:
        try:
            os.kill(pid,0)
        except ProcessLookupError:
            return
        time.sleep(.02)
    raise RuntimeError(f"Worker {pid} survived its cancellation/deadline")


def lifecycle(bins, work, env):
    sleeper = work / "sleeper"
    sleeper.write_text("#!/bin/sh\nprintf '%s' \"$$\" > pid\nprintf 'started\\n' >> attempts\nexec sleep 60\n")
    sleeper.chmod(0o700)
    for mode in ("cancel", "shutdown", "crash"):
        state=work / (mode+" state")
        with listener(bins,work,env,state,[sleeper],("-timeout","3s")) as (process,url):
            task=call(bins,work,env,url,"send",request("wait",True),75)
            task_work=workspace(state,task);wait_for(task_work/"pid");pid=int((task_work/"pid").read_text())
            if mode=="cancel":
                canceled=call(bins,work,env,url,"cancel",{"id":task["id"]})
                require(canceled["status"]["state"]=="TASK_STATE_CANCELED","Cancellation lost its status")
                gone(pid)
            elif mode=="crash":
                process.kill();process.wait(timeout=5)
            else:
                process.terminate();process.wait(timeout=10);gone(pid)
        with listener(bins,work,env,state,[sleeper],("-timeout","3s")) as (_,url):
            retained=call(bins,work,env,url,"get",{"id":task["id"]},1 if mode=="cancel" else 125)
            if mode!="cancel":
                require(retained.get("metadata",{}).get("bench/outcome")=="unknown","Restart manufactured an execution outcome")
            gone(pid)
            require((task_work/"attempts").read_text()=="started\n","Restart resubmitted a worker")
    print("ok A2A executable cancellation, graceful stop, abrupt restart, retained uncertainty, no rerun",flush=True)


def agent_composition(bins, work, env):
    expert=work / "expert"; (expert/"bin").mkdir(parents=True)
    (expert/"AGENTS.md").write_text("A2A_EXPERT_CONTEXT. Write the requested output, then let the check judge it.\n")
    check=expert/"bin/check"; check.write_text("#!/bin/sh\ntest -f result.txt && test \"$(cat result.txt)\" = verified\n");check.chmod(0o700)
    seen=[]
    def respond(request):
        seen.append(request)
        return "```ply\nprintf 'verified\\n' > result.txt\n```" if len(seen)%2 else "Result verified."
    state=work / "agent state"
    with harness.model_fixture(env,respond) as (fixture_env,calls):
        with listener(bins,work,fixture_env,state,[bins/"agent","run","-no-cage","-m","openai/fixture",
                 "-C",".","-state","../state","-evidence","../control",expert],
                 ("-pass-env","OPENAI_API_KEY","-pass-env","OPENAI_BASE_URL","-artifact","result.txt")) as (_,url):
            tasks=[]
            for marker in ("FIRST_TASK_PRIVATE","SECOND_TASK_PRIVATE"):
                tasks.append(call(bins,work,fixture_env,url,"send",request(marker+". Produce verified in result.txt.")))
            require(len(calls)==4,"Agent did not use its real Ask/Ply/check loop")
            second=json.dumps(seen[2]);require("FIRST_TASK_PRIVATE" not in second and "SECOND_TASK_PRIVATE" in second,"Remote tasks shared a model context")
            require(tasks[0]["contextId"]!=tasks[1]["contextId"],"Remote Agent reused context IDs")
            for task in tasks:
                require((workspace(state,task)/"result.txt").read_bytes()==b"verified\n","Agent's checked artifact missing")
                require(any(p.get("filename")=="result.txt" for a in task["artifacts"] for p in a["parts"]),"A2A did not export checked artifact")
    sessions=list(state.glob("workspaces/*/control/runs/*.jsonl"));require(len(sessions)==2,"Separate Agent evidence missing")
    for session in sessions:
        invoke([bins/"ask","replay","-check",session],cwd=work,env=env)
    print("ok A2A -> Tend -> Agent -> Ask/Ply: checked files, fresh model contexts, independently verified evidence",flush=True)


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument("--bin-dir",required=True,type=Path);args=parser.parse_args();bins=args.bin_dir.resolve()
    for name in ("a2a","a2aserve","tend","agent","ask","brief","ply"):
        require((bins/name).is_file(),"Missing executable "+name)
    with tempfile.TemporaryDirectory(prefix="bench-a2a-integration-") as temp:
        work=Path(temp).resolve();home=work/"home";home.mkdir()
        env={"HOME":str(home),"TMPDIR":str(work),"PATH":str(bins)+os.pathsep+os.defpath,"LANG":"C","LC_ALL":"C",
             "XDG_CONFIG_HOME":str(home/"config"),"XDG_STATE_HOME":str(home/"state")}
        lifecycle(bins,work,env);agent_composition(bins,work,env)

if __name__=="__main__":
    try:
        main()
    except (RuntimeError,OSError,ValueError,subprocess.SubprocessError) as error:
        print("a2a integration: "+str(error),file=sys.stderr);sys.exit(1)
