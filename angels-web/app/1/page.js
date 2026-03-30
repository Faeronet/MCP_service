"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'


import Pic1 from '../../public/pictures/pic1.jpg'


import styles from '../../app/case.module.css'
import styles1 from '@/app/1/repo1.module.scss';


const StoryContent = ({
  useResponsiveOffset = true
}) => {
  const classNameFirstColumn = cx({
    'cds--col-lg-13': true,
    'cds--offset-lg-3': useResponsiveOffset
  });
  const content = <Grid>
     <Column sm={{
    span: 4,
    offset: 0
  }} md={{
    span: 8,
    offset: 0
  }} lg={{
    span: 12,
    offset: 4
  }} >
    <div>
          
 <p>
 <h2 style={{
          margin: '0 0 30px'
        }}>Проект Путь Ангелов посвящаем во благо всех осознающих существ - в качестве НАШЕЙ им помощи в освобождении от помрачений сознания и страданий.</h2>

 Принцип работы с проектом :   

 <p  align="left"> 1.	Необходимо найти ситуацию которую нужно проработать и определить помощника и время для проработки ситуации . </p>

 <p  align="left"> 2.	Использую вот такую молитву : </p> 

 <p align="left">  Во имя Бога, Во имя Духа ,  Я ЕСМЬ ТО ЧТО Я  ЕСМЬ, </p>

 <p align="left">  Во имя Ангела  ( Указываем имя ) , </p>

 <p align="left">  Во имя Ангела Хранителя  </p>

 <p align="left"> Принимаю исцеление ситуации:”описываем ситуацию ” .  </p>

 <p align="left"> 3.	Благодарю Ангела(Указываем имя), </p>

 <p align="left"> 4.	Благодарю Ангела Хранителя  , </p>
 
 <p align="left"> 5.	Благодарю Бога . </p>

 <p align="left"> Вместо слова Бога можно говорить Творца . В зависимости от ощущений и желания . </p>



</p>
   
      </div>
      </Column>
      </Grid>;
  const style = {
    height: '100%'
  };
  if (useResponsiveOffset) {
    style.margin = '0';
    style.width = '100%';
  }


  
 return <div>
	{content}
	</div>;


};

export default function RepoPage() {
  return(<div className={styles1.backgroundContainer1}>
    <StoryContent/>
  </div>);
}