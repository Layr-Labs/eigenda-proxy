# See https://github.com/numtide/treefmt-nix?tab=readme-ov-file#flakes
{ pkgs, ... }:
{
  # Used to find the project root
  projectRootFile = "flake.nix";
  # List of formatters to use
  programs.actionlint.enable = true;
  programs.gofmt.enable = true;
  programs.nixfmt-rfc-style.enable = true;
}
