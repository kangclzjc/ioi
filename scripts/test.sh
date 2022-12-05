#!/bin/bash -e

ARGUMENT_LIST=(
  "num"
  "bw"
  "arg-three"
)

podNum=3

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


function rand(){
    min=$1
    max=$(($2 - $min + 1))
    num=$(($RANDOM+1000000000))
    echo $(($num%$max + $min))
}

generateCore() {
  newYaml=$1.yml
  cp template.yml $newYaml
  sed -i -e "s;%podName%;$1;g" $newYaml
  sed -i -e "s;%containerName%;$2;g" $newYaml
  sed -i -e "s;%filename%;$3;g" $newYaml
  sed -i -e "s;%iodepth%;$4;g" $newYaml
  sed -i -e "s;%rw%;$5;g" $newYaml
  sed -i -e "s;%ioengine%;$6;g" $newYaml
  sed -i -e "s;%bs%;$7;g" $newYaml
  sed -i -e "s;%size%;$8;g" $newYaml
  sed -i -e "s;%rate%;$9;g" $newYaml
  sed -i -e "s;%numjobs%;${10};g" $newYaml
  sed -i -e "s;%runtime%;${11};g" $newYaml
  sed -i -e "s;%name%;${12};g" $newYaml
}

generatePodSpecs() {
  total=$bw
  minBw=$(( $bw / $podNum / 2 ))
  maxBw=$(( $bw / $podNum))

  for ((i=1; i<$podNum;i++))
  do
    echo "generate "$i" pod"
    local podName containerName filename iodepth rw ioengine bs size rate numjobs runtime name
    podName=fio-$i
    newYaml=$podName.yml
    cp template.yml $newYaml
    containerName=con-$i
    filename=/tmp/test
    iodepth=1
    rw=randwrite
    ioengine=libaio
    bs=1k
    size=10g
    rate=$(rand minBw maxBw)
    echo $rate
    total=$(($total - $rate))

    numjobs=1
    runtime=604800
    name=test-$i
    generateCore $podName $containerName $filename $iodepth $rw $ioengine $bs $size $rate $numjobs $runtime $name
  done

  echo "generate $podNum pod"
  podName=fio-$podNum
  newYaml=$podName.yml
  cp template.yml $newYaml
  containerName=con-$podNum
  name=test-$podNum
  echo $total

  generateCore $podName $containerName $filename $iodepth $rw $ioengine $bs $size $total $numjobs $runtime $name
}

generatePodSpecs