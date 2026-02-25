# **tidyup**

tidyup is a lightweight Go-based CLI tool to identify and clean up unused Python virtual environments. It is specifically optimized for macOS and users of the uv package manager who find their systems cluttered with transient environments.

## **Features**

* **Advanced Activity Detection**: Ported from an older project I did in Python, [`uv-tidy`](https://github.com/fblissjr/uv-tidy/), it inspects pyvenv.cfg and activation scripts for actual usage timestamps, rather than relying on unreliable directory access times.  
* **Concurrent Scanning**: Uses Go goroutines to calculate directory sizes in parallel, significantly speeding up scans of large SSDs.  
* **System-Wide Awareness**: With the --system flag, it automatically includes standard uv cache locations (`~/Library/Caches/uv/venvs`, etc.).  
* **Safe Deletion**: Features an interactive confirmation prompt and summarizes exactly how much space will be reclaimed before touching any files.  
* **Customizable Depth**: Limit recursion to avoid scanning deep into system or project folders.

## **Installation**

1. **Clone the repository**:  
   `git clone [https://github.com/fblissjr/tidyup.git](https://github.com/fblissjr/tidyup.git)`
   `cd tidyup`

2. **Build and install**:  
   `make install`

   *Note: This moves the binary to /usr/local/bin, requiring sudo privileges.*

## **Usage**

### **Common Commands**

**Scan current directory for venvs unused for 30+ days**:

`tidyup`

**Scan entire home directory including uv caches, limited to 5 levels deep**:

`tidyup -system -depth 5 ~`

**Identify environments over 60 days old and delete them**:

`tidyup -age 60 -delete ~`

### **macOS & uv Specific Examples**

Since uv often manages environments in central cache locations on macOS, you can target those specifically to reclaim massive amounts of space:

**Preview all global uv environments unused for 2 months**:

`tidyup -system -age 60 ~/Library/Caches/uv/venvs`

**Find and remove the largest dead environments in your Home folder**:

Sorts by size automatically. Review the list, then run with -delete.
`findvenv -system -age 90 ~ | tail -n 10`

**Deep scan of a specific dev workspace**:

`findvenv -depth 10 ~/dev/projects`

### **Safe Deletion**

When using the `-delete` flag, tidyup will:

1. List all candidates for removal with their size and age.  
2. Provide a total count and total reclaimed space estimate.  
3. Prompt for an explicit y/N confirmation before executing os.RemoveAll.

## **Technical Notes**

* **Permissions**: Ensure you have proper permissions for the directories you are scanning.  
* **Pruning**: To maintain performance, the tool aggressively prunes `.git`, `node_modules`, and macOS `Library` folders from its search path.  
* **uv Optimization**: Standard `uv` layouts are detected using the `pyvenv.cfg` marker which contains specific metadata.

## **License**

This project is licensed under the MIT License -- see the LICENSE file for details.
