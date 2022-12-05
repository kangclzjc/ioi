#!/bin/bash -e

ARGUMENT_LIST=(
  "num"
  "bw"
  "arg-three"
)


getParams() {
  # read arguments
  opts=$(getopt \
    --longoptions "$(printf "%s:," "${ARGUMENT_LIST[@]}")" \
    --name "$(basename "$0")" \
    --options "" \
    -- "$@"
  )
  eval set --$opts
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --num)
        podNum=$2
        shift 2
        ;;

      --bw)
        bw=$2
        shift 2
        ;;

      --arg-three)
        argThree=$2
        shift 2
        ;;

      *)
        break
        ;;
    esac
  done
}

getParams "$@"
echo $bw

generatePodSpecs() {
  for i in $(seq 1 $podNum)
  do
    echo "generate "$i" pod"

  done
}

generatePodSpecs