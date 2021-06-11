{
  description = "HashiCorp Waypoint SDK";

  inputs.waypoint.url = "github:hashicorp/waypoint";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, flake-utils, waypoint }:
    flake-utils.lib.eachDefaultSystem (system: {
        # Just use the exact same shell environment as Waypoint.
        devShell = waypoint.devShell.${system};
      }
    );
}
