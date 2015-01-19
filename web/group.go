package web

import (
	"net/http"

	"github.com/hfurubotten/autograder/git"
)

func requestrandomgrouphandler(w http.ResponseWriter, r *http.Request) {
	// Checks if the user is signed in and a teacher.
	member, err := checkMemberApproval(w, r, false)
	if err != nil {
		http.Error(w, err.Error(), 404)
		log.Println(err)
		return
	}

	orgname := r.FormValue("course")
	if !git.HasOrganization(orgname) {
		http.Error(w, "Does not have organization.", 404)
	}

	org := git.NewOrganization(orgname)

	org.PendingRandomGroup[member.Username] = nil
	org.StickToSystem()
}

func newgrouphandler(w http.ResponseWriter, r *http.Request) {
	// Checks if the user is signed in and a teacher.
	member, err := checkMemberApproval(w, r, false)
	if err != nil {
		http.Error(w, err.Error(), 404)
		log.Println(err)
		return
	}

	course := r.FormValue("course")

	if _, ok := member.Courses[course]; !ok {
		pages.RedirectTo(w, r, "/", 307)
		log.Println("Unknown course.")
		return
	}

	org := git.NewOrganization(course)
	org.GroupCount = org.GroupCount + 1

	group, err := git.NewGroup(course, org.GroupCount)
	if err != nil {
		pages.RedirectTo(w, r, "/", 307)
		log.Println("Couldn't make new group object.")
		return
	}

	r.ParseForm()
	members := r.PostForm["member"]

	var user git.Member
	var opt git.CourseOptions
	for _, username := range members {
		user = git.NewMemberFromUsername(username)
		opt = user.Courses[course]
		opt.IsGroupMember = true
		opt.GroupNum = org.GroupCount
		user.Courses[course] = opt
		user.StickToSystem()
		group.AddMember(username)
	}

	org.PendingGroup[org.GroupCount] = nil
	org.StickToSystem()
	group.StickToSystem()

	if member.IsTeacher {
		pages.RedirectTo(w, r, "/course/teacher/"+org.Name+"#groups", 307)
	} else {
		pages.RedirectTo(w, r, "/course/"+org.Name+"#groups", 307)
	}
}

type approvegroupview struct {
	ErrorMsg string
	Error    bool
	Approved bool
	ID       int
}

func approvegrouphandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	view := approvegroupview{Error: true}
	// Checks if the user is signed in and a teacher.
	member, err := checkTeacherApproval(w, r, false)
	if err != nil {
		http.Error(w, err.Error(), 404)
		log.Println(err)
		return
	}

	groupID, err := strconv.Atoi(r.FormValue("groupid"))
	if err != nil {
		view.ErrorMsg = err.Error()
		err = enc.Encode(view)
		if err != nil {
			log.Println(err)
		}
		return
	}

	orgname := r.FormValue("course")

	group, err := git.NewGroup(orgname, groupID)
	if err != nil {
		view.ErrorMsg = err.Error()
		err = enc.Encode(view)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if group.Active {
		view.ErrorMsg = "This group is already active."
		err = enc.Encode(view)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if len(group.Members) <= 1 {
		view.ErrorMsg = "No members in this group."
		err = enc.Encode(view)
		if err != nil {
			log.Println(err)
		}
		return
	}

	_, ok1 := member.Teaching[orgname]
	_, ok2 := member.AssistantCourses[orgname]

	if !ok1 && !ok2 {
		view.ErrorMsg = "You are not teaching this course."
		err = enc.Encode(view)
		if err != nil {
			log.Println(err)
		}
		return
	}

	org := git.NewOrganization(orgname)

	if org.GroupAssignments > 0 {
		repo := git.RepositoryOptions{
			Name:     "group" + r.FormValue("groupid"),
			Private:  org.Private,
			AutoInit: true,
			Hook:     true,
		}
		err = org.CreateRepo(repo)
		if err != nil {
			log.Println(err)
			view.ErrorMsg = "Error communicating with Github. Couldn't create repository."
			enc.Encode(view)
			return
		}

		newteam := git.TeamOptions{
			Name:       "group" + r.FormValue("groupid"),
			Permission: git.PERMISSION_PUSH,
			RepoNames:  []string{"group" + r.FormValue("groupid")},
		}

		teamID, err := org.CreateTeam(newteam)
		if err != nil {
			log.Println(err)
			view.ErrorMsg = "Error communicating with Github. Can't create team."
			enc.Encode(view)
			return
		}

		for username, _ := range group.Members {
			err = org.AddMemberToTeam(teamID, username)
			if err != nil {
				log.Println(err)
				view.ErrorMsg = "Error communicating with Github. Can't add member to team."
				enc.Encode(view)
				return
			}
		}
	}

	delete(org.PendingGroup, groupID)
	org.StickToSystem()

	group.Active = true
	group.StickToSystem()

	view.Error = false
	view.Approved = true
	view.ID = groupID
	err = enc.Encode(view)
	if err != nil {
		log.Println(err)
	}
}