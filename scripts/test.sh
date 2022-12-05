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

function rwmod() {
	randrw=$(rand 0 2)
    if [ $randrw -eq 0 ]
    then
    	rw=randread
    elif [ $randrw -eq 1 ]
    then
    	rw=randwrite
    else
    	rw=randrw
    fi
    echo $rw
}

function getRates() {
	rates=(0 0)
	if [ $1 == "randread" ]; then
		rates[0]=$(rand minBw maxBw)
	elif [ $1 == "randwrite" ]; then
		rates[1]=$(rand minBw maxBw)
	else
		rates[0]=$(rand minBw maxBw)
		rates[1]=$(rand minBw maxBw)
	fi
	echo ${rates[*]}
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
  sed -i -e "s;%rRate%;$9;g" $newYaml
  sed -i -e "s;%wRate%;${10};g" $newYaml
  sed -i -e "s;%numjobs%;${11};g" $newYaml
  sed -i -e "s;%runtime%;${12};g" $newYaml
  sed -i -e "s;%name%;${13};g" $newYaml
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

    rw=`rwmod`
    ioengine=libaio
    bs=1k
    size=10g
    rates=($(getRates $rw))
    eval set $rates
    echo $rates
    total=$(($total - ${rates[0]}))

    numjobs=1
    runtime=604800
    name=test-$i
    generateCore $podName $containerName $filename $iodepth $rw $ioengine $bs $size ${rates[0]} ${rates[1]} $numjobs $runtime $name
  done

  echo "generate $podNum pod"
  podName=fio-$podNum
  newYaml=$podName.yml
  cp template.yml $newYaml
  containerName=con-$podNum
  name=test-$podNum
  rw=`rwmod`
  echo $total

  generateCore $podName $containerName $filename $iodepth $rw $ioengine $bs $size $total $total $numjobs $runtime $name
}

generatePodSpecs
