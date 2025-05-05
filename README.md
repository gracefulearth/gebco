# pixi_gebco

Tools for creating and modifying a PIXI format GEBCO dataset.

## New from Scratch: Order of Operations

First, convert the GEBCO `.tif` files to `.pixi` files using the `gtiff2pixi` command.

Second, stitch the converted GEBCO `.pixi` tiles into one giant super-`.pixi` using the `stich-pixi` command.

Third, verify the stitched file against the GEBCO `.tif` files for accuracy using the `verify` command.


