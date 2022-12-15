#!/bin/bash

ARGUMENT_LIST=(
  "num"
  "bw"
  "type"
)

podNum=3

declare -A coefficients
coefficients["512b"]=10
coefficients["1k"]=8
coefficients["4k"]=5
coefficients["8k"]=3
coefficients["16k"]=2
coefficients["32k"]=1

declare -A rclass
declare -A wclass
QOS=("high-prio" "medium-prio" "low-prio")
rclass["high-prio"]=200
rclass["medium-prio"]=100
rclass["low-prio"]=50

wclass["high-prio"]=200
wclass["medium-prio"]=100
wclass["low-prio"]=50

#default coefficient is 1
rcoefficient=1
wcoefficient=1
BS=("512b" "1k" "4k" "8k" "16k" "32k")
bw=1000
BE_BW=500
TYPE=GA

getParams() {
  # read arguments
  opts=$(
    getopt \
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

    --type)
      TYPE=$2
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

function rand() {
  min=$1
  max=$(($2 - $min + 1))
  num=$(($RANDOM + 1000000000))
  echo $(($num % $max + $min))
}

function rwmod() {
  randrw=$(rand 0 2)
  if [ $randrw -eq 0 ]; then
    rw=randread
  elif [ $randrw -eq 1 ]; then
    rw=randwrite
  else
    rw=randrw
  fi
  echo $rw
}

# random generate randread/randwrite bw
function getRates() {
  rates=(0 0)
  if [ $1 = "randread" ]; then
    rates[0]=$(rand minBw maxBw)
  elif [ $1 = "randwrite" ]; then
    rates[1]=$(rand minBw maxBw)
  else
    rates[0]=$(rand minBw maxBw)
    rates[1]=$(rand minBw maxBw)
  fi
  echo ${rates[*]}
}

# get a fixed bw workload
function getFixedRates() {
  rates=(0 0)
  if [ $1 = "randread" ]; then
    rates[0]=$2
  elif [ $1 = "randwrite" ]; then
    rates[1]=$2
  else
    bw1=$(expr $2 / 4)
    bw2=$(expr $2 / 2)
    rates[0]=$(rand $bw1 $bw2)
    rates[1]=$(expr $2 - ${rates[0]})
  fi
  echo ${rates[*]}
}

function randBs() {
  i=$(rand 1 33)
  if [ $i == 33 ]; then
    echo 512
  else
    echo $i"k"
  fi
}

function getCoefficient() {
  num=${1%?}
  for ((i = 1; i < ${#BS[@]}; i++)); do
    if [ $num -lt ${BS[i]%?} ]; then
      echo ${coefficients[${BS[$(expr $i - 1)]}]}
      return
    fi
  done
  echo 1
}

generateGACore() {
  newYaml=$1.yml
  cp GA_template.yml $newYaml
  echo $@
  sed -i -e "s;%podName%;$1;g" $newYaml
  sed -i -e "s;%containerName%;$2;g" $newYaml
  sed -i -e "s;%filename%;$3;g" $newYaml
  sed -i -e "s;%iodepth%;$4;g" $newYaml
  sed -i -e "s;%rw%;$5;g" $newYaml
  sed -i -e "s;%ioengine%;$6;g" $newYaml
  sed -i -e "s;%rbs%;$7;g" $newYaml
  sed -i -e "s;%wbs%;$8;g" $newYaml
  sed -i -e "s;%size%;$9;g" $newYaml
  sed -i -e "s;%rRate%;${10};g" $newYaml
  sed -i -e "s;%rbps%;${10};g" $newYaml
  sed -i -e "s;%wRate%;${11};g" $newYaml
  sed -i -e "s;%wbps%;${11};g" $newYaml
  sed -i -e "s;%numjobs%;${12};g" $newYaml
  sed -i -e "s;%runtime%;${13};g" $newYaml
  sed -i -e "s;%name%;${14};g" $newYaml
  sed -i -e "s;%prio%;${15};g" $newYaml
}

generateBECore() {
  newYaml=$1.yml
  cp BE_template.yml $newYaml
  echo $@
  sed -i -e "s;%podName%;$1;g" $newYaml
  sed -i -e "s;%containerName%;$2;g" $newYaml
  sed -i -e "s;%filename%;$3;g" $newYaml
  sed -i -e "s;%iodepth%;$4;g" $newYaml
  sed -i -e "s;%rw%;$5;g" $newYaml
  sed -i -e "s;%ioengine%;$6;g" $newYaml
  sed -i -e "s;%rbs%;$7;g" $newYaml
  sed -i -e "s;%wbs%;$8;g" $newYaml
  sed -i -e "s;%size%;$9;g" $newYaml
  sed -i -e "s;%rRate%;${10};g" $newYaml
  sed -i -e "s;%rbps%;${10};g" $newYaml
  sed -i -e "s;%wRate%;${11};g" $newYaml
  sed -i -e "s;%wbps%;${11};g" $newYaml
  sed -i -e "s;%numjobs%;${12};g" $newYaml
  sed -i -e "s;%runtime%;${13};g" $newYaml
  sed -i -e "s;%name%;${14};g" $newYaml
  sed -i -e "s;%prio%;${15};g" $newYaml
}

generateGAPodSpecs() {
  total=$bw
  minBw=$(($bw / $podNum / 2))
  maxBw=$(($bw / $podNum))

  for ((i = 1; i < $podNum; i++)); do
    echo "generate "$i" pod"
    local podName containerName filename iodepth rw ioengine rbs wbs size rate numjobs runtime name
    podName=fio-$i
    newYaml=$podName.yml
    cp GA_template.yml $newYaml
    containerName=con-$i
    filename=/tmp/test
    iodepth=1

    rw=$(rwmod)
    ioengine=libaio
    rbs=($(randBs))
    wbs=($(randBs))

    size=10g
    rates=($(getRates $rw))
    rcoefficient=$(getCoefficient $rbs)
    wcoefficient=$(getCoefficient $wbs)
    eval set $rates
    total=$(($total - ${rates[0]} - ${rates[1]}))

    rates[0]=$((${rates[0]} / $rcoefficient))
    rates[1]=$((${rates[1]} / $wcoefficient))
    echo ${rates[0]} ${rates[1]}

    numjobs=1
    runtime=604800
    name=test-$i
    generateGACore $podName $containerName $filename $iodepth $rw $ioengine $rbs $wbs $size ${rates[0]} ${rates[1]} $numjobs $runtime $name
  done

  echo "generate $podNum pod"
  podName=fio-$podNum
  newYaml=$podName.yml
  cp GA_template.yml $newYaml
  containerName=con-$podNum
  name=test-$podNum
  rw=$(rwmod)
  rbs=($(randBs))
  wbs=($(randBs))
  remain=$(($total))
  local rates
  rates=($(getFixedRates $rw $total))
  rcoefficient=$(getCoefficient $rbs)
  wcoefficient=$(getCoefficient $wbs)
  rates[0]=$((${rates[0]} / $rcoefficient))
  rates[1]=$((${rates[1]} / $wcoefficient))
  generateGACore $podName $containerName $filename $iodepth $rw $ioengine $rbs $wbs $size ${rates[0]} ${rates[1]} $numjobs $runtime $name
}

function generateBEPodSpecs() {
  i=1
  while (($BE_BW >= 0)); do
    # generate a class and then update GA_BW
    echo "generate $i pod"
    t=$(rand 0 $((${#QOS[@]} - 1)))
    local podName containerName filename iodepth rw ioengine rbs wbs size rate numjobs runtime name
    podName=fio-be-$i
    newYaml=$podName.yml
    cp BE_template.yml $newYaml
    containerName=con-$i
    BE_BW=$(($BE_BW - 100))
    filename=/tmp/test
    iodepth=1

    rw=randrw
    ioengine=libaio
    rbs=32k
    wbs=32k

    size=10g
    echo $t
    rates[0]=${rclass[${QOS[$t]}]}
    rates[1]=${wclass[${QOS[$t]}]}
    echo ${rates[0]} ${rates[1]}
    rcoefficient=1
    wcoefficient=1

    BE_BW=$(($BE_BW - ${rates[0]} - ${rates[1]}))

    numjobs=1
    runtime=604800
    name=test-$i
    prio=${QOS[$t]}
    echo $prio
    generateBECore $podName $containerName $filename $iodepth $rw $ioengine $rbs $wbs $size ${rates[0]} ${rates[1]} $numjobs $runtime $name $prio
    #		# generate spec
    let i=i+1
  done
}

function main() {
  if [ $TYPE = "GA" ]; then
    generateGAPodSpecs
  else
    generateBEPodSpecs
  fi
}
main
