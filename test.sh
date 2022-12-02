#!/bin/bash -e

ARGUMENT_LIST=(
  "arg-one"
  "arg-two"
  "arg-three"
)


# read arguments
opts=$(getopt \
  --longoptions "$(printf "%s:," "${ARGUMENT_LIST[@]}")" \
  --name "$(basename "$0")" \
  --options "" \
  -- "$@"
)
echo $opts
eval set --$opts
echo $opts
while [[ $# -gt 0 ]]; do
  case "$1" in
    --arg-one)
      argOne=$2
      shift 2
      ;;

    --arg-two)
      argTwo=$2
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

echo $argOne