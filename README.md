# mineraddrctl

Utility program to assist with controlling miner owner, worker, and control addresses.

Basically, by passing in the miner actor address you can avoid the requirement to have
the miner running when performing the following operations:
 - Change owner address
 - Change worker address
 - Set control addresses

## Usage

```
$ mineraddrctl -h
NAME:
   mineraddrctl - manage miner addresses

USAGE:
   mineraddrctl [global options] command [command options] [arguments...]

COMMANDS:
   owner    
   worker   
   control  
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

## Example 

To start an owner transfer, run the following command:
```
$ mineraddrctl owner transfer --really-do-it --miner f012345 <new_owner_addr> <current_owner_addr>
```