package sql

import (
	"bbutil_cli/server/models"
	"bbutil_cli/server/util"
)

type TaskExecuteDao struct{}

// 查询组内整包或替换任务列表
// 查询当前最近的构建任务信息
func (t *TaskExecuteDao) List(groupId, taskType string) ([]models.TaskExecute, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var tasks []models.TaskExecute

	tx = tx.Table("task_info").Where("task_info.group_id = ? AND task_info.type = ? AND task_info.status = 1", groupId, taskType)
	tx = tx.Select(`
		task_info.id AS task_id,
		task_info.code AS task_code,
		task_info.value AS task_value,
		task_execute.id,
		COALESCE(emp_info.chinese_name, emp_info.username) AS emp_name,
		task_execute.start_time,
		task_execute.consume_time,
		task_execute.jira,
		task_execute.tester_name,
		task_execute.tester_code,
		task_execute.date,
		task_execute.execute_status,
		task_execute.desc,
		task_execute.build_id
	`)
	tx = tx.Joins(`
	LEFT JOIN(
		SELECT
			*
		FROM
			(
			SELECT
				* ,
				ROW_NUMBER () OVER (PARTITION BY task_id
			ORDER BY
				start_time DESC) AS row_num
			FROM
				task_execute te 
	)
		WHERE
			row_num = 1
	)task_execute ON task_execute .task_id = task_info .id
	`)
	tx = tx.Joins(`
	LEFT JOIN emp_info ON emp_info.id = task_execute.emp_id
	`).Order("task_execute.start_time desc").Find(&tasks)

	return tasks, tx.Commit().Error
}

// select by primarykey
func (t *TaskExecuteDao) SelectByPrimaryKey(taskId string) (models.TaskExecute, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var task models.TaskExecute
	tx.Table("task_execute").Where("task_execute.id = ? AND task_execute.status = 1", taskId).Find(&task)
	return task, tx.Commit().Error
}

// select by buildId
func (t *TaskExecuteDao) SelectByBuildId(buildId string) (models.TaskExecute, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var task models.TaskExecute
	tx.Table("task_execute").Where("task_execute.build_id = ? AND task_execute.status = 1", buildId).Find(&task)
	return task, tx.Commit().Error
}

// select by task_id and date
func (t *TaskExecuteDao) SelectByTaskIdAndDate(taskId, date string) (models.TaskExecute, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var task models.TaskExecute
	tx.Table("task_execute").Where("task_execute.task_id = ? AND task_execute.date = ? AND task_execute.status = 1", taskId, date).Order("strftime('%Y-%m-%d %H:%M:%S', start_time) desc").First(&task)
	return task, tx.Commit().Error
}

func (d *TaskExecuteDao) Create(taskExecute models.TaskExecute) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("task_execute").Create(&taskExecute).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *TaskExecuteDao) Update(taskExecute models.TaskExecute) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	tx = tx.Table("task_execute")
	tx = tx.Where("task_execute.id = ?", taskExecute.Id)
	if taskExecute.ConsumeTime != 0 {
		tx = tx.UpdateColumn("consume_time", taskExecute.ConsumeTime)
	}
	if taskExecute.ExecuteStatus != 0 {
		tx = tx.UpdateColumn("execute_status", taskExecute.ExecuteStatus)
	}
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *TaskExecuteDao) CleanOverDueExecuteInfo() int64 {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	result := tx.Table("task_execute").Where(" date(start_time) < date('now', '-7 days')").Delete(&models.TaskInfo{})
	deleteCount := result.RowsAffected
	tx.Commit()
	return deleteCount
}
