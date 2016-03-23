select avg(score) as avg_score, shop_id
from 2016_03_22_goods -- 日期
where category=50010850 -- 限定连衣裙
and added_at > '2016-02-25' -- 上新范围
group by shop_id -- 按档口分组
having count(*) > 5 -- 连衣裙数量
order by avg_score desc -- 平均分最高
limit 50;
