"""Run a project's preview generator independently of the editor window."""
import contextlib
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile
import time
import traceback


def write_json(path, value):
    temp = path.with_name(path.name + "." + str(os.getpid()) + ".tmp")
    temp.write_text(json.dumps(value), encoding="utf-8")
    os.replace(temp, path)


def read_json(path, default):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return default


@contextlib.contextmanager
def lock(path, blocking=True):
    # Kernel-owned locks are released even if Python or the editor crashes.
    with path.open("a+b") as file:
        file.seek(0)
        if os.name == "nt":
            import msvcrt
            if path.stat().st_size == 0:
                file.write(b"\0")
                file.flush()
            while True:
                try:
                    file.seek(0)
                    msvcrt.locking(file.fileno(), msvcrt.LK_NBLCK, 1)
                    break
                except OSError:
                    if not blocking:
                        yield False
                        return
                    time.sleep(.05)
            try:
                yield True
            finally:
                file.seek(0)
                msvcrt.locking(file.fileno(), msvcrt.LK_UNLCK, 1)
        else:
            import fcntl
            try:
                fcntl.flock(file, fcntl.LOCK_EX | (0 if blocking else fcntl.LOCK_NB))
            except BlockingIOError:
                yield False
                return
            try:
                yield True
            finally:
                fcntl.flock(file, fcntl.LOCK_UN)


def generate(root, environment):
    script = root / "tools/ship_previews/generate_ship_previews.py"
    # Run the project's maintained implementation, including its smoothing fixes.
    namespace = runpy.run_path(str(script))
    settings = namespace["main"].__globals__
    output = root / "voidcrew/modules/ship_upgrades/previews"
    output.parent.mkdir(parents=True, exist_ok=True)
    if not os.environ.get("DMM_TOOLS"):
        tool = shutil.which("dmm-tools")
        if tool:
            os.environ["DMM_TOOLS"] = tool
    with tempfile.TemporaryDirectory(prefix=".strongdmm-previews-", dir=output.parent) as temp:
        stage = Path(temp)
        settings["OUTPUT_DIR"] = stage
        settings["ENVIRONMENT"] = str(environment)
        namespace["main"]()
        manifest = read_json(stage / "manifest.json", None)
        if not isinstance(manifest, dict) or not isinstance(manifest.get("hulls"), dict) or not isinstance(manifest.get("modules"), dict):
            raise RuntimeError("The generator did not produce a valid preview manifest")
        files = sorted(stage.glob("*.png")) + [stage / "manifest.json"]
        output.mkdir(parents=True, exist_ok=True)
        backup = stage / "backup"
        backup.mkdir()
        replaced = []
        try:
            for source in files:
                target = output / source.name
                existed = target.exists()
                if existed:
                    shutil.copy2(target, backup / source.name)
                os.replace(source, target)
                replaced.append((target, existed))
        except BaseException:
            for target, existed in reversed(replaced):
                if existed:
                    os.replace(backup / target.name, target)
                else:
                    target.unlink(missing_ok=True)
            raise
    print("Purchase previews updated.", flush=True)


def request(folder, root, environment):
    queue = folder / "request.json"
    state = folder / "status.json"
    # Queue changes and worker shutdown share a short lock, so a save arriving
    # as the current worker exits cannot be stranded. Multiple editors share it.
    guard = lock(folder / "queue.lock")
    guard.__enter__()
    worker = None
    try:
        previous = read_json(queue, {"revision": 0})
        revision = previous["revision"] + 1
        write_json(queue, {"revision": revision, "root": str(root), "environment": str(environment)})
        worker = lock(folder / "worker.lock", blocking=False)
        if not worker.__enter__():
            worker.__exit__(None, None, None)
            worker = None
            status = read_json(state, {})
            status["queued"] = True
            status["updated"] = time.time()
            write_json(state, status)
            return
        while True:
            item = read_json(queue, {})
            revision = item["revision"]
            write_json(state, {"phase": "running", "queued": False, "pid": os.getpid(), "updated": time.time()})
            guard.__exit__(None, None, None)
            guard = None
            log_path = folder / "generation.log"
            with log_path.open("w", encoding="utf-8") as log:
                flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
                result = subprocess.run(
                    [sys.executable, "-B", "-u", __file__, "--run", item["root"], item["environment"]],
                    cwd=item["root"], stdout=log, stderr=log, creationflags=flags,
                )
            guard = lock(folder / "queue.lock")
            guard.__enter__()
            if read_json(queue, {})["revision"] != revision:
                continue
            write_json(state, {"phase": "complete" if result.returncode == 0 else "failed",
                              "queued": False, "updated": time.time()})
            return
    finally:
        # Release the worker while still holding the queue lock.
        if worker is not None:
            worker.__exit__(None, None, None)
        if guard is not None:
            guard.__exit__(None, None, None)


if __name__ == "__main__":
    if sys.argv[1] == "--run":
        generate(Path(sys.argv[2]), Path(sys.argv[3]))
    else:
        folder = Path(sys.argv[1])
        try:
            request(folder, Path(sys.argv[2]), Path(sys.argv[3]))
        except BaseException:
            with (folder / "generation.log").open("a", encoding="utf-8") as log:
                traceback.print_exc(file=log)
            write_json(folder / "status.json", {"phase": "failed", "queued": False, "updated": time.time()})
            raise
