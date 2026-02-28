// scripts/smithery-build.js
const { execSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const platform = os.platform();
const arch = os.arch();

let url = "";

if (platform === 'win32') {
  url = "https://github.com/halflifezyf2680/mpm-vibe-coding/releases/latest/download/mpm-windows-amd64.zip";
} else if (platform === 'darwin') {
  if (arch === 'arm64') {
    url = "https://github.com/halflifezyf2680/mpm-vibe-coding/releases/latest/download/mpm-darwin-arm64.tar.gz";
  } else {
    url = "https://github.com/halflifezyf2680/mpm-vibe-coding/releases/latest/download/mpm-darwin-amd64.tar.gz";
  }
} else {
  url = "https://github.com/halflifezyf2680/mpm-vibe-coding/releases/latest/download/mpm-linux-amd64.tar.gz";
}

console.log("Downloading native binary from: " + url);

try {
  if (platform === 'win32') {
    // Windows 自带 curl 和 tar
    execSync(`curl -L "${url}" -o mpm.zip`, { stdio: 'inherit' });
    execSync(`tar -xf mpm.zip`, { stdio: 'inherit' });
    fs.renameSync("mpm-windows-amd64", "mpm-bin");
    // 清理临时文件
    if (fs.existsSync("mpm.zip")) {
      fs.unlinkSync("mpm.zip");
    }
  } else {
    execSync(`curl -L "${url}" -o mpm.tar.gz`, { stdio: 'inherit' });
    execSync(`tar -xzf mpm.tar.gz`, { stdio: 'inherit' });
    
    // 动态寻找解压出来的文件夹
    const files = fs.readdirSync('.');
    for (const f of files) {
      if (f.startsWith('mpm-') && f !== 'mpm.tar.gz' && fs.statSync(f).isDirectory()) {
        fs.renameSync(f, "mpm-bin");
        break;
      }
    }
    // 赋权
    execSync(`chmod +x mpm-bin/mpm-go`, { stdio: 'inherit' });
    // 清理临时文件
    if (fs.existsSync("mpm.tar.gz")) {
      fs.unlinkSync("mpm.tar.gz");
    }
  }
} catch (error) {
  console.error("Failed to build/download binary:", error);
  process.exit(1);
}
