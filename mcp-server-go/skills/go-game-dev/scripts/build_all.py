import os
import subprocess
import shutil
import sys

# Go Game Cross-Compilation Script
# Supported Targets: Windows, Linux, macOS, Web (Wasm)

APP_NAME = "mygame"
OUTPUT_DIR = "bin"

TARGETS = [
    {"os": "windows", "arch": "amd64", "ext": ".exe", "flags": "-H windowsgui"},
    {"os": "linux", "arch": "amd64", "ext": "", "flags": ""},
    {"os": "darwin", "arch": "amd64", "ext": "", "flags": ""}, # macOS Intel
    {"os": "darwin", "arch": "arm64", "ext": "", "flags": ""}, # macOS Silicon
    {"os": "js", "arch": "wasm", "ext": ".wasm", "flags": ""}
]

def build():
    if not os.path.exists(OUTPUT_DIR):
        os.makedirs(OUTPUT_DIR)
        
    print(f"🚀 Starting build for {APP_NAME}...")
    
    for t in TARGETS:
        env = os.environ.copy()
        env["GOOS"] = t["os"]
        env["GOARCH"] = t["arch"]
        
        output_name = f"{APP_NAME}_{t['os']}_{t['arch']}{t['ext']}"
        output_path = os.path.join(OUTPUT_DIR, output_name)
        
        # Base commands
        cmd = ["go", "build", "-o", output_path]
        
        # Add ldflags for production (strip debug info, hide console on Windows)
        # -s: disable symbol table
        # -w: disable DWARF generation
        ldflags = "-s -w"
        if t["flags"]:
            ldflags += " " + t["flags"]
        
        cmd.extend(["-ldflags", ldflags])
        
        # Handle package path (assuming main is in current dir or cmd/game)
        # Adjust this if your main.go is elsewhere
        if os.path.exists("cmd/game/main.go"):
            cmd.append("./cmd/game")
        else:
            cmd.append(".")

        print(f"📦 Building {output_name}...")
        try:
            subprocess.run(cmd, env=env, check=True)
            print(f"   ✅ Success: {output_path}")
            
            # Special handling for Wasm: Need checking HTML wrapper
            if t["os"] == "js":
                copy_wasm_exec(env)
                
        except subprocess.CalledProcessError as e:
            print(f"   ❌ Failed: {e}")

def copy_wasm_exec(env):
    # Try to find wasm_exec.js in GOROOT
    goroot = subprocess.check_output(["go", "env", "GOROOT"], env=env).decode().strip()
    wasm_js = os.path.join(goroot, "misc", "wasm", "wasm_exec.js")
    
    if os.path.exists(wasm_js):
        dest = os.path.join(OUTPUT_DIR, "wasm_exec.js")
        shutil.copy(wasm_js, dest)
        print(f"   📄 Copied wasm_exec.js to {dest}")
        
        # Create a simple index.html for testing
        html_content = """<!DOCTYPE html>
<script src="wasm_exec.js"></script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("APP_NAME.wasm"), go.importObject).then((result) => {
        go.run(result.instance);
    });
</script>
""".replace("APP_NAME.wasm", f"{APP_NAME}_js_wasm.wasm")
        
        with open(os.path.join(OUTPUT_DIR, "index.html"), "w") as f:
            f.write(html_content)
        print("   📄 Generated index.html")

if __name__ == "__main__":
    build()
    print("✨ Build process completed.")
