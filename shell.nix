{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = with pkgs; [
    go
    tmux # scripts/pty-smoke.sh
    mpv # voice/round playback
    chafa # photo viewer
  ];
}
