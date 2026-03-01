import os
import shutil
import pathlib
import sys

RELEASE_DIR_NAME = "mpm-release"

# 设置 UTF-8 编码输出
if sys.platform == "win32":
    import codecs

    sys.stdout = codecs.getwriter("utf-8")(sys.stdout.buffer, "strict")
    sys.stderr = codecs.getwriter("utf-8")(sys.stderr.buffer, "strict")


def package_mpm():
    # 动态获取当前脚本所在目录作为根目录
    root = pathlib.Path(__file__).parent.resolve()

    release_root = root / RELEASE_DIR_NAME
    dist = release_root / "MyProjectManager"

    # 1. 如果 release_root 已存在，先清理（确保干净）
    if release_root.exists():
        shutil.rmtree(release_root)

    # 创建多级目录
    dist.mkdir(parents=True)

    print(f"🚀 开始打包 MyProjectManager (Base: {root})...")
    print(f"📂 目标路径: {dist}")

    # 定义需要包含的核心文件夹
    # 注意: mcp-server-go 包含完整的服务代码 (含 skills 目录)
    core_dirs = [
        "mcp-server-go",  # 当前核心服务 (包含 skills/)
        "docs",  # 图片和额外文档
    ]

    # 定义需要包含的核心根目录文件
    core_files = [
        "README.md",
        "README_EN.md",
        "install.ps1",
        "package_product.py",
        "QUICKSTART.md",
        "QUICKSTART_EN.md",
        "docs/images/mpm_logo.png",  # Logo 已移至此处
    ]

    # 定义需要包含的编译脚本
    build_scripts = [
        "scripts/build-windows.ps1",
        "scripts/build-unix.sh",
        "scripts/build-cross-platform.sh",
    ]

    # 2. 复制文件夹 (带逻辑过滤)
    for dname in core_dirs:
        src_dir = root / dname
        target_dir = dist / dname

        if src_dir.exists():
            print(f"📦 正在打包模块: {dname}...")
            # 过滤掉不需要的垃圾
            # 注意: target 是 rust 编译目录，通常很大且非必需（除非我们从里面拿exe）
            # 我们假设exe已经移动到了 bin 目录
            shutil.copytree(
                src_dir,
                target_dir,
                ignore=shutil.ignore_patterns(
                    "__pycache__",
                    ".mcp-data",
                    ".git",
                    "*.pyc",
                    ".vscode",
                    ".idea",
                    "target",
                    "node_modules",
                    "debug_*",
                    "check_*",
                    "*.pdb",
                    "*.log",
                ),
            )
        else:
            print(f"⚠️ 警告: 目录不存在 {dname}")

    # 2.5. 特殊处理 user-manual：只保留 COMPLETE-MANUAL-CONCISE.md
    user_manual_src = root / "user-manual"
    user_manual_dst = dist / "user-manual"
    if user_manual_src.exists():
        print(f"📦 正在打包模块: user-manual (仅保留 COMPLETE-MANUAL-CONCISE.md)...")
        user_manual_dst.mkdir(parents=True)
        concise_manual = user_manual_src / "COMPLETE-MANUAL-CONCISE.md"
        if concise_manual.exists():
            shutil.copy2(concise_manual, user_manual_dst / "COMPLETE-MANUAL-CONCISE.md")
            print(f"✅ 已复制: COMPLETE-MANUAL-CONCISE.md")
        else:
            print(f"⚠️ 警告: COMPLETE-MANUAL-CONCISE.md 不存在")
    else:
        print(f"⚠️ 警告: user-manual 目录不存在")

    # 3. 复制根目录文件
    for fname in core_files:
        src_file = root / fname
        if src_file.exists():
            print(f"📄 正在打包文件: {fname}...")
            shutil.copy2(src_file, dist / fname)
        else:
            print(f"⚠️ 警告: 文件不存在 {fname}")

    # 3.5. 复制编译脚本
    scripts_dst = dist / "scripts"
    scripts_dst.mkdir(parents=True, exist_ok=True)
    for script in build_scripts:
        src_script = root / script
        if src_script.exists():
            print(f"📄 正在打包编译脚本: {script}...")
            shutil.copy2(src_script, scripts_dst / src_script.name)
        else:
            print(f"⚠️ 警告: 编译脚本不存在 {script}")

    # 4. 验证关键二进制文件
    required_bins = [
        "mcp-server-go/bin/mpm-go.exe",
        "mcp-server-go/bin/ast_indexer.exe",
    ]

    print("\n🔍 正在校验二进制完整性...")
    all_exist = True
    for bin_rel in required_bins:
        bin_path = dist / bin_rel
        if not bin_path.exists():
            print(f"❌ 缺失: {bin_rel} (可能导致功能不全)")
            all_exist = False
        else:
            size_mb = bin_path.stat().st_size / (1024 * 1024)
            print(f"✅ 存在: {bin_rel} ({size_mb:.1f} MB)")

    if not all_exist:
        print(f"\n⚠️ 警告: 部分二进制文件缺失，请先编译项目！")
        return

    print(f"\n✨ 大功告成！发布包已生成: {dist.absolute()}")
    print(f"👉 只需将此文件夹拷贝到目标机器即可使用。")


if __name__ == "__main__":
    package_mpm()
