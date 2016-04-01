select avg(score) as avg_score, a.shop_id, name
from 2016_03_24_goods a -- 日期
left join 2016_03_24_shops b -- 日期
on a.shop_id = b.shop_id
where category=50010850 -- 限定连衣裙
and added_at > '2016-03-01' -- 上新范围
and a.shop_id < 20000 -- 老店
group by shop_id -- 按档口分组
having count(*) > 10 -- 连衣裙数量
order by avg_score desc -- 平均分最高
limit 30;
