create table if not exists folders (
	id int not null AUTO_INCREMENT,
	images_count int not null default 0,
	total_size bigint not null default 0, /* excluding copies */
	created_at datetime not null default current_timestamp(),

	primary key(id)
) AUTO_INCREMENT=1000;

