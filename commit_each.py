import os
import subprocess

def git_add_commit(directory):
    print(f"Processing {directory}...")
    cwd = directory
    # Get all untracked files and modified files
    cmd = ["git", "ls-files", "--others", "--modified", "--exclude-standard"]
    result = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    files = result.stdout.splitlines()
    
    for f in files:
        # Ignore .md files explicitly just in case
        if f.endswith(".md"):
            continue
            
        print(f"Committing {f} in {directory}")
        subprocess.run(["git", "add", f], cwd=cwd)
        subprocess.run(["git", "commit", "-m", f"Add/Update {f}"], cwd=cwd)

if __name__ == "__main__":
    # ai_engine is its own repo
    git_add_commit("/home/wyckie/Desktop/MyProjects/ERPAgent/ai_engine")
    
    # frontend and backend are in the root repo
    git_add_commit("/home/wyckie/Desktop/MyProjects/ERPAgent")
